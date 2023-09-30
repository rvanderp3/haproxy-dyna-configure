FROM registry.access.redhat.com/ubi8/go-toolset:1.19.10-10.1692783630


USER root
RUN yum install -y bind-utils net-tools procps jq
RUN wget https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.13.10/openshift-client-linux.tar.gz
RUN tar xf openshift-client-linux.tar.gz -C /usr/local/bin


WORKDIR /usr/src/app

COPY . .
RUN ls -lt
RUN go mod tidy && go mod vendor

COPY run.sh .

RUN ./hack/build.sh
CMD ./run.sh
