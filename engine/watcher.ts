/** File watcher for hot-reload during development */

export interface WatcherOptions {
  dir: string;
  extensions?: string[];
  debounceMs?: number;
  onChange: () => void | Promise<void>;
}

/**
 * Watch a directory for file changes and trigger a callback.
 * Debounces rapid changes to avoid excessive reloads.
 */
export async function watchFiles(opts: WatcherOptions): Promise<void> {
  const extensions = opts.extensions ?? [".ts", ".js", ".yaml", ".json"];
  const debounceMs = opts.debounceMs ?? 200;

  let timer: number | undefined;

  const watcher = Deno.watchFs(opts.dir, { recursive: true });

  console.log(`Watching ${opts.dir} for changes...`);

  for await (const event of watcher) {
    // Only care about file modifications and creations
    if (event.kind !== "modify" && event.kind !== "create") continue;

    // Filter by extension
    const relevant = event.paths.some((p) => extensions.some((ext) => p.endsWith(ext)));
    if (!relevant) continue;

    // Debounce
    if (timer !== undefined) clearTimeout(timer);
    timer = setTimeout(async () => {
      const changedFiles = event.paths
        .filter((p) => extensions.some((ext) => p.endsWith(ext)))
        .map((p) => p.replace(opts.dir, "."));
      console.log(`\nFiles changed: ${changedFiles.join(", ")}`);
      console.log("Reloading...");

      try {
        await opts.onChange();
        console.log("Reload complete.");
      } catch (err) {
        console.error("Reload error:", err);
      }
    }, debounceMs);
  }
}
