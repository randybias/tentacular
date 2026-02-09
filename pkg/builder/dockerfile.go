package builder

// GenerateDockerfile produces the Dockerfile for a pipedreamer engine-only container.
// The workflow code will be mounted separately via ConfigMap.
func GenerateDockerfile() string {
	return `FROM denoland/deno:distroless

WORKDIR /app

# Copy engine
COPY .engine/ /app/engine/

# Copy deno.json for import map resolution
COPY .engine/deno.json /app/deno.json

# Cache engine dependencies
RUN ["deno", "cache", "engine/main.ts"]

# Set DENO_DIR for runtime caching of node dependencies
ENV DENO_DIR=/tmp/deno-cache

EXPOSE 8080

ENTRYPOINT ["deno", "run", "--no-lock", "--unstable-net", "--allow-net", "--allow-read=/app,/var/run/secrets", "--allow-write=/tmp", "--allow-env", "engine/main.ts", "--workflow", "/app/workflow/workflow.yaml", "--port", "8080"]
`
}
