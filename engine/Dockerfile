FROM denoland/deno:distroless

WORKDIR /app

# Copy engine source
COPY engine/ /app/engine/

# Copy import map
COPY engine/deno.json /app/deno.json

# Copy lockfile for dependency integrity
COPY engine/deno.lock /app/deno.lock

# Cache engine dependencies at build time (locked)
RUN ["deno", "cache", "--lock=deno.lock", "engine/main.ts"]

EXPOSE 8080

ENTRYPOINT ["deno", "run", "--no-lock", "--unstable-net", "--allow-net", "--allow-read=/app,/var/run/secrets", "--allow-write=/tmp", "--allow-env", "engine/main.ts", "--workflow", "/app/workflow/workflow.yaml", "--port", "8080"]
