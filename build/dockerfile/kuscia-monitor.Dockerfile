ARG ROOT_DIR="/home"
ARG PROM_IMAGE="prom/prometheus:v2.45.3"
ARG GRAFANA_IMAGE="grafana/grafana:10.3.1"
FROM ${PROM_IMAGE} as prom
FROM ${GRAFANA_IMAGE} as grafana
FROM openanolis/anolisos:8.8
ENV TZ=Asia/Shanghai
RUN yum install -y jq &&  yum clean all
COPY --from=prom /bin/prometheus ${ROOT_DIR}/bin
COPY --from=grafana /usr/share/grafana/ /usr/share/grafana/
COPY --from=grafana /etc/grafana /etc/grafana/
COPY --from=grafana /var/lib/grafana /var/lig/grafana/
COPY --from=grafana /var/log/grafana /var/log/grafana/

RUN mkdir -p /home/config
RUN mkdir -p /var/lib/grafana/dashboards/
COPY scripts/templates/kuscia-monitor-datasource.yaml /etc/grafana/provisioning/datasources/
COPY scripts/templates/grafana-dashboard-machine.json /var/lib/grafana/dashboards/machine.json
COPY scripts/deploy/init_kuscia_monitor.sh /home
ENV PATH="${PATH}:${ROOT_DIR}/bin:/bin/aux:/usr/share/grafana/bin"
WORKDIR ${ROOT_DIR}
RUN chmod +x /home/init_kuscia_monitor.sh
CMD ["/home/init_kuscia_monitor.sh"]
ENTRYPOINT ["/bin/bash", "--"]
