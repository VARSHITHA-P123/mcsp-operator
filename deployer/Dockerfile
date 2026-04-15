FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Install required tools
RUN microdnf install -y git tar gzip && microdnf clean all

# Install oc CLI
RUN curl -L https://mirror.openshift.com/pub/openshift-v4/clients/ocp/latest/openshift-client-linux.tar.gz -o /tmp/oc.tar.gz && \
    tar -xzf /tmp/oc.tar.gz -C /usr/local/bin oc && \
    rm /tmp/oc.tar.gz && \
    chmod +x /usr/local/bin/oc

# Create kube config directory with correct permissions
RUN mkdir -p /.kube && chmod 777 /.kube

USER 1001