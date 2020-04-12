FROM registry.bryk.io/general/shell:0.1.0

# Metadata
ARG VERSION
LABEL maintainer="Ben Cessa <ben@pixative.com>"
LABEL version=${VERSION}

# Expose required ports and volumes
EXPOSE 9090
VOLUME /data

# Add application binary and use it as default entrypoint
COPY covid-tracking_linux_amd64 /bin/covid-tracking
ENTRYPOINT ["/bin/covid-tracking"]
