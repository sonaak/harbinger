FROM alpine:latest

LABEL maintainer="evilwire <knight.w.fu@gmail.com>"

# Set the three basic meta info for app
ARG VERSION
ARG GHASH
ARG BUILD_TIME
ENV VERSION ${VERSION}
ENV GHASH ${GHASH}
ENV BUILD_TIME ${BUILD_TIME}

# Set the working directory
ENV WORK_DIR /opt/sonaak/
WORKDIR $WORK_DIR

# Copy the binary, and make it runnable
COPY build/${VERSION}/sparrow ${WORK_DIR}/sparrow
RUN chmod 500 ${WORK_DIR}

# Set the entrypoint
ENTRYPOINT ["/opt/sonaak/sparrow"]
