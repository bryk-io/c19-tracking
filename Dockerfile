FROM registry.bryk.io/general/shell:0.1.0

# Metadata
ARG VERSION
LABEL maintainer="Ben Cessa <ben@pixative.com>"
LABEL version=${VERSION}

# Expose required ports and volumes
EXPOSE 9090
VOLUME /etc/ct19
VOLUME /home/guest/tls

# Add application binary and use it as default entrypoint
COPY ct19_linux_amd64 /bin/ct19
ENTRYPOINT ["/bin/ct19"]
