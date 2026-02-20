import type { Context } from "tentacular";
import type { SynthesizeOutput } from "./synthesize.ts";

export interface PostReviewOutput {
  review_id: number;
  review_url: string;
  verdict: string;
}

/**
 * Post the synthesized PR review to GitHub via the PR Review API.
 *
 * Creates a single review with inline comments in one API call.
 * GitHub supports up to ~4000 chars per review body and 100 inline comments.
 */
export default async function run(ctx: Context, input: unknown): Promise<PostReviewOutput> {
  const review = input as SynthesizeOutput;

  ctx.log.info(
    `Posting ${review.verdict} review to ${review.owner}/${review.repo} PR#${review.pr_number}`,
  );

  const github = ctx.dependency("github");
  const auth = `Bearer ${github.secret}`;

  // GitHub PR Review API:
  // POST /repos/{owner}/{repo}/pulls/{pull_number}/reviews
  const body = {
    commit_id: review.commit_id,
    body: review.review_body,
    event: review.verdict, // "APPROVE" | "COMMENT" | "REQUEST_CHANGES"
    comments: review.inline_comments.map((c) => ({
      path: c.path,
      line: c.line,
      body: c.body,
      // Use "RIGHT" side for new code in the diff
      side: "RIGHT",
    })),
  };

  const res = await github.fetch!(
    `/repos/${review.owner}/${review.repo}/pulls/${review.pr_number}/reviews`,
    {
      method: "POST",
      headers: {
        Authorization: auth,
        Accept: "application/vnd.github+json",
        "content-type": "application/json",
      },
      body: JSON.stringify(body),
    },
  );

  if (!res.ok) {
    const errBody = await res.text();
    // 422 often means the line number doesn't exist in the diff — retry without inline comments
    if (res.status === 422 && review.inline_comments.length > 0) {
      ctx.log.warn(
        `Review with inline comments failed (422), retrying without inline comments`,
      );
      const fallbackBody = { ...body, comments: [] };
      const fallbackRes = await github.fetch!(
        `/repos/${review.owner}/${review.repo}/pulls/${review.pr_number}/reviews`,
        {
          method: "POST",
          headers: {
            Authorization: auth,
            Accept: "application/vnd.github+json",
            "content-type": "application/json",
          },
          body: JSON.stringify(fallbackBody),
        },
      );
      if (!fallbackRes.ok) {
        throw new Error(`GitHub PR Review API fallback error: ${fallbackRes.status} ${await fallbackRes.text()}`);
      }
      const fallbackData = await fallbackRes.json() as Record<string, unknown>;
      ctx.log.info(`Review posted (no inline comments) — id=${fallbackData["id"]}`);
      return {
        review_id: Number(fallbackData["id"] ?? 0),
        review_url: String(fallbackData["html_url"] ?? ""),
        verdict: review.verdict,
      };
    }
    throw new Error(`GitHub PR Review API error: ${res.status} ${errBody}`);
  }

  const data = await res.json() as Record<string, unknown>;
  const reviewId = Number(data["id"] ?? 0);
  const reviewUrl = String(data["html_url"] ?? "");

  ctx.log.info(`Review posted — id=${reviewId} verdict=${review.verdict} url=${reviewUrl}`);

  return { review_id: reviewId, review_url: reviewUrl, verdict: review.verdict };
}
