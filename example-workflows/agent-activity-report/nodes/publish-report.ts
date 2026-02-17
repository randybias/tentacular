import type { Context } from "tentacular";

interface PublishResult {
  uploaded: boolean;
  reportUrl: string;
}

function formatBlobTimestamp(iso: string): string {
  return iso.replace(/[-:]/g, "").replace("T", "-").replace(/\.\d+Z$/, "");
}

/** Upload HTML report to Azure Blob Storage */
export default async function run(ctx: Context, input: unknown): Promise<PublishResult> {
  const report = input as { html: string; title: string; summary: string };

  const azure = ctx.dependency("azure-blob");
  if (!azure.secret) {
    ctx.log.warn("No azure credentials, skipping upload");
    return { uploaded: false, reportUrl: "" };
  }

  const blobName = `agent-report-${formatBlobTimestamp(new Date().toISOString())}.html`;
  const baseUrl = ctx.config.azure_blob_base_url as string;
  const blobPath = `/sep-reports/${blobName}`;
  const publicUrl = `${baseUrl}/${blobName}`;

  ctx.log.info(`Uploading report to ${publicUrl}`);

  // azure.fetch!() is scoped to the dependency host -- pass path only, not full URL
  const res = await azure.fetch!(`${blobPath}?${azure.secret}`, {
    method: "PUT",
    headers: {
      "Content-Type": "text/html; charset=utf-8",
      "x-ms-blob-type": "BlockBlob",
    },
    body: report.html,
  });

  if (!res.ok) {
    ctx.log.error(`Azure upload failed: ${res.status} ${res.statusText}`);
    return { uploaded: false, reportUrl: "" };
  }

  ctx.log.info(`Uploaded agent activity report: ${publicUrl}`);
  return { uploaded: true, reportUrl: publicUrl };
}
