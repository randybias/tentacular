FROM denoland/deno:alpine

WORKDIR /app

# Copy engine source
COPY engine/ /app/engine/

# Copy import map
COPY engine/deno.json /app/deno.json

# Copy lockfile for dependency integrity
COPY engine/deno.lock /app/deno.lock

# Cache engine dependencies at build time (locked)
RUN deno cache --lock=deno.lock engine/main.ts

# Copy startup script
COPY engine/start.sh /app/start.sh
RUN chmod +x /app/start.sh

EXPOSE 8080

ENTRYPOINT ["/bin/sh", "/app/start.sh"]
