FROM scratch
COPY --chmod=0755 dist/folkweave /usr/local/bin/folkweave
# Copy data as a child directory so its own mode is applied by older builders.
COPY --chown=65532:65532 --chmod=0700 packaging/rootfs/ /
USER 65532:65532
ENV FOLKWEAVE_DATA_DIR=/data FOLKWEAVE_BASE_URL=http://localhost:8084
VOLUME /data
EXPOSE 8084
ENTRYPOINT ["/usr/local/bin/folkweave"]
CMD ["serve", "--listen", "0.0.0.0:8084"]
