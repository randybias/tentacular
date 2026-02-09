package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateDockerfile produces the Dockerfile for a pipedreamer workflow container.
// It scans the nodes directory to generate explicit deno cache commands for each node.
func GenerateDockerfile(workflowDir string) string {
	cacheLines := ""
	nodesDir := filepath.Join(workflowDir, "nodes")
	entries, err := os.ReadDir(nodesDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".ts") {
				cacheLines += fmt.Sprintf("RUN [\"deno\", \"cache\", \"--no-lock\", \"nodes/%s\"]\n", e.Name())
			}
		}
	}

	return `FROM denoland/deno:distroless

WORKDIR /app

# Copy engine
COPY .engine/ /app/engine/

# Copy workflow files
COPY workflow.yaml /app/
COPY nodes/ /app/nodes/

# Copy deno.json for import map resolution
COPY .engine/deno.json /app/deno.json

# Cache engine dependencies
RUN ["deno", "cache", "engine/main.ts"]

# Cache node dependencies (third-party imports)
` + cacheLines + `
EXPOSE 8080

ENTRYPOINT ["deno", "run", "--no-lock", "--unstable-net", "--allow-net", "--allow-read=/app,/var/run/secrets", "--allow-write=/tmp", "--allow-env", "engine/main.ts", "--workflow", "/app/workflow.yaml", "--port", "8080"]
`
}
