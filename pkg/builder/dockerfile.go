package builder

// GenerateDockerfile produces the Dockerfile for a pipedreamer workflow container.
func GenerateDockerfile() string {
	return `FROM denoland/deno:distroless

WORKDIR /app

# Copy engine
COPY .engine/ /app/engine/

# Copy workflow files
COPY workflow.yaml /app/
COPY nodes/ /app/nodes/

# Copy deno.json for import map resolution
COPY .engine/deno.json /app/deno.json

# Cache dependencies
RUN ["deno", "cache", "engine/main.ts"]

EXPOSE 8080

ENTRYPOINT ["deno", "run", "--allow-net", "--allow-read=/app", "--allow-write=/tmp", "engine/main.ts", "--workflow", "/app/workflow.yaml", "--port", "8080"]
`
}
