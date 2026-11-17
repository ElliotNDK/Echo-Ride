FROM quay.io/keycloak/keycloak:26.8.2 AS builder
COPY keycloak.conf /opt/keycloak/conf/keycloak.conf
RUN /opt/keycloak/bin/kc.sh build

FROM quay.io/keycloak/keycloak:26.8.2
COPY --from=builder /opt/keycloak/ /opt/keycloak/
ENTRYPOINT ["/opt/keycloak/bin/kc.sh"]
CMD ["start", "--optimized"]