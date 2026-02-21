import type { Context } from "tentacular";
import type { PrContext } from "./fetch-pr.ts";
import type { SemgrepScanOutput } from "./semgrep-scan.ts";
import type { DepReviewOutput } from "./dep-review.ts";
import type { CheckRunsOutput } from "./check-runs.ts";
import type { CodeScanOutput } from "./code-scan.ts";

/** An inline comment on a specific file/line */
export interface InlineComment {
  path: string;
  line: number;
  body: string;
}

/** Full synthesized review output — passed to post-review */
export interface SynthesizeOutput {
  // Pass-through for post-review
  owner: string;
  repo: string;
  pr_number: number;
  commit_id: string;
  // Review content
  review_body: string;
  inline_comments: InlineComment[];
  verdict: "APPROVE" | "COMMENT" | "REQUEST_CHANGES";
}

/** Fan-in input: one key per parent node name */
interface SynthesizeInput {
  "fetch-pr": PrContext;
  "semgrep-scan": SemgrepScanOutput;
  "dep-review": DepReviewOutput;
  "check-runs": CheckRunsOutput;
  "code-scan": CodeScanOutput;
}

/** OpenAI chat completions API response (minimal subset) */
interface OpenAIResponse {
  choices: Array<{ message: { content: string } }>;
}

/**
 * Synthesize all scanner findings into a unified GitHub PR review using Claude.
 *
 * Receives merged output from all 5 parent nodes via Tentacular's fan-in semantics.
 * Calls Anthropic Claude API and returns a structured review with verdict and inline comments.
 */
export default async function run(ctx: Context, input: unknown): Promise<SynthesizeOutput> {
  const data = input as SynthesizeInput;
  const pr = data["fetch-pr"];
  const semgrep = data["semgrep-scan"];
  const deps = data["dep-review"];
  const checks = data["check-runs"];
  const codeql = data["code-scan"];

  ctx.log.info(
    `Synthesizing review for ${pr.owner}/${pr.repo} PR#${pr.pr_number} — ` +
    `semgrep=${semgrep.alerts.length}, codeql=${codeql.alerts.length}, ` +
    `deps=${deps.vulnerabilities.length}, checks=${checks.overall_status}`,
  );

  // --- Build prompt ---
  const semgrepSection = semgrep.alerts.length === 0
    ? "None found."
    : semgrep.alerts.map((a) =>
        `- [${a.severity.toUpperCase()}] ${a.rule_id}: ${a.message}\n  File: ${a.path}:${a.line}`
      ).join("\n");

  const codeqlSection = codeql.alerts.length === 0
    ? "None found."
    : codeql.alerts.map((a) =>
        `- [${a.severity.toUpperCase()}] ${a.rule_id}: ${a.message}\n  File: ${a.path}:${a.line}`
      ).join("\n");

  const depsSection =
    deps.vulnerabilities.length === 0 && deps.license_concerns.length === 0
      ? "No issues found."
      : [
          deps.vulnerabilities.length > 0
            ? "VULNERABILITIES:\n" + deps.vulnerabilities.map((v) =>
                v.vulnerabilities.map((vuln) =>
                  `- [${vuln.severity.toUpperCase()}] ${v.package_name}: ${vuln.advisory_summary} (${vuln.advisory_ghsa_id})`
                ).join("\n")
              ).join("\n")
            : "",
          deps.license_concerns.length > 0
            ? "LICENSE CONCERNS:\n" + deps.license_concerns.map((l) =>
                `- ${l.package_name}: ${l.license} (in ${l.manifest_path})`
              ).join("\n")
            : "",
        ].filter(Boolean).join("\n\n");

  const checksSection =
    checks.overall_status === "none"
      ? "No CI checks configured."
      : `Overall: ${checks.overall_status.toUpperCase()}\n` +
        (checks.failed_checks.length > 0
          ? `Failed: ${checks.failed_checks.join(", ")}`
          : `All ${checks.checks.length} checks passed.`);

  const changedFilesList = pr.changed_files
    .map((f) => `  ${f.status}: ${f.filename} (+${f.additions} -${f.deletions})`)
    .join("\n");

  const prompt = `You are a senior engineer performing a code review on a GitHub pull request.
You have been given automated static analysis results to help focus your review.
Write a concise, actionable review — not a verbose summary of inputs.

## PR: ${pr.pr_title}
URL: ${pr.pr_url}

**Description:**
${pr.pr_body || "(no description)"}

**Changed files (${pr.changed_files.length}):**
${changedFilesList}

**Diff:**
\`\`\`diff
${pr.diff_summary || "(no diff available)"}
\`\`\`

---

## Automated Scan Results

### Semgrep (SAST)
${semgrepSection}

### CodeQL (SAST)
${codeqlSection}

### Dependency Review (CVEs + Licenses)
${depsSection}

### CI Check Runs
${checksSection}

---

## Your Task

Write a GitHub PR review with exactly this JSON structure (respond ONLY with JSON, no other text):

{
  "verdict": "APPROVE" | "COMMENT" | "REQUEST_CHANGES",
  "review_body": "string — 2-4 paragraph review summary. Be direct. Don't repeat every finding — synthesize. Call out the most important issues. If nothing significant, say so.",
  "inline_comments": [
    {
      "path": "relative/path/to/file.ts",
      "line": <line number>,
      "body": "concise, actionable comment"
    }
  ]
}

Rules:
- Inline comments only for real issues (security, correctness, logic bugs). Max 8.
- Use REQUEST_CHANGES if: security vulnerability, CVE in new dependency, CI failure, significant logic bug
- Use COMMENT if: style, minor improvements, questions, non-blocking concerns
- Use APPROVE if: no significant issues found
- Prioritize: security > correctness > performance > style`;

  // --- Call OpenAI API ---
  const openai = ctx.dependency("openai");

  const res = await openai.fetch!("/v1/chat/completions", {
    method: "POST",
    headers: {
      "Authorization": `Bearer ${openai.secret}`,
      "content-type": "application/json",
    },
    body: JSON.stringify({
      model: "gpt-4o",
      max_tokens: 4096,
      messages: [{ role: "user", content: prompt }],
    }),
  });

  if (!res.ok) {
    throw new Error(`OpenAI API error: ${res.status} ${await res.text()}`);
  }

  const completionRaw = await res.json() as Record<string, unknown>;
  const choices = completionRaw["choices"] as OpenAIResponse["choices"] | undefined;
  const rawText = choices?.[0]?.message?.content ?? "";

  // Parse Claude's JSON response
  let parsed: { verdict: string; review_body: string; inline_comments: InlineComment[] };

  if (!rawText) {
    // Mock context — no real AI response available, return placeholder
    ctx.log.warn("No AI response (mock context) — returning placeholder synthesis");
    return {
      owner: pr.owner,
      repo: pr.repo,
      pr_number: pr.pr_number,
      commit_id: pr.head_sha,
      review_body: "(placeholder — AI unavailable in mock context)",
      inline_comments: [],
      verdict: "COMMENT",
    };
  }

  try {
    // Model may wrap JSON in a ```json``` block — strip it
    const jsonStr = rawText.replace(/^```(?:json)?\s*/m, "").replace(/\s*```$/m, "").trim();
    parsed = JSON.parse(jsonStr);
  } catch (err) {
    ctx.log.error("Failed to parse AI response as JSON:", rawText.slice(0, 200));
    throw new Error(`AI returned invalid JSON: ${err}`);
  }

  // Validate verdict
  const validVerdicts = ["APPROVE", "COMMENT", "REQUEST_CHANGES"] as const;
  const verdict = validVerdicts.includes(parsed.verdict as typeof validVerdicts[number])
    ? (parsed.verdict as SynthesizeOutput["verdict"])
    : "COMMENT";

  ctx.log.info(`Synthesis complete — verdict: ${verdict}, inline_comments: ${parsed.inline_comments?.length ?? 0}`);

  return {
    owner: pr.owner,
    repo: pr.repo,
    pr_number: pr.pr_number,
    commit_id: pr.head_sha,
    review_body: parsed.review_body ?? "(no review body)",
    inline_comments: (parsed.inline_comments ?? []).slice(0, 8),
    verdict,
  };
}
