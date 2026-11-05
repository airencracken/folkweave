FROM scratch
COPY --chmod=0755 dist/folkweave /usr/local/bin/folkweave
COPY --chown=65532:65532 --chmod=0700 packaging/data/ /data/
USER 65532:65532
ENV FOLKWEAVE_DATA_DIR=/data FOLKWEAVE_BASE_URL=http://localhost:8084
VOLUME /data
EXPOSE 8084
ENTRYPOINT ["/usr/local/bin/folkweave"]
CMD ["serve", "--listen", "0.0.0.0:8084"]
