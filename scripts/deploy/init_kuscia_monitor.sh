#!/bin/bash
# Copyright 2023 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

nohup prometheus --config.file=/home/config/prometheus.yml > /dev/null 2>&1 &
GRAFANA_URL="http://admin:admin@localhost:3000/api/datasources"
EXPECTED_STATUS_CODE=200
nohup grafana server --homepath=/usr/share/grafana --config=/etc/grafana/grafana.ini --packaging=docker cfg:default.log.mode=console cfg:default.paths.data=/var/lib/grafana cfg:default.paths.logs=/var/log/grafana cfg:default.paths.plugins=/var/lib/grafana/plugins cfg:default.paths.provisioning=/etc/grafana/provisioning > /dev/null 2>&1 &
while true; do
    status_code=$(curl -w "%{http_code}" -o /dev/null -s ${GRAFANA_URL})
    if [ "$status_code" -eq "$EXPECTED_STATUS_CODE" ]; then
        echo "Grafana has been started: $status_code"
        break
    else
        echo "Grafana is starting: $status_code"
    fi
    sleep 2
done

datasource_uid=$(curl -s http://admin:admin@localhost:3000/api/datasources | jq -r '.[] | select(.name == "Kuscia-monitor") | .uid')
kill "$(pgrep grafana)"
sed -i "s/{{Kuscia-datasource}}/${datasource_uid}/g" /var/lib/grafana/dashboards/machine.json
sed -i '1n;s/#//g' /usr/share/grafana/conf/provisioning/dashboards/sample.yaml
cp /usr/share/grafana/conf/provisioning/dashboards/sample.yaml /etc/grafana/provisioning/dashboards/sample.yaml
nohup grafana server --homepath=/usr/share/grafana --config=/etc/grafana/grafana.ini --packaging=docker cfg:default.log.mode=console cfg:default.paths.data=/var/lib/grafana cfg:default.paths.logs=/var/log/grafana cfg:default.paths.plugins=/var/lib/grafana/plugins cfg:default.paths.provisioning=/etc/grafana/provisioning > /dev/null 2>&1 &
trap "exit 0" SIGTERM
while true; do sleep 1; done
