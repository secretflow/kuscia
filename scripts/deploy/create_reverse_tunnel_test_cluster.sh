#!/bin/bash
#
# Copyright 2025 Ant Group Co., Ltd.
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
#

function docker_get_ctrs() {
  local prefix=$1
  docker ps | grep "$prefix" | awk '{print $1}'
}

function build_replica_conn() {
  local hostname
  hostname=$(hostname -I | awk '{print $1}')

  read -ra alice_ctrs <<< "$(docker_get_ctrs kuscia-autonomy-alice)"
  read -ra bob_ctrs <<< "$(docker_get_ctrs kuscia-autonomy-bob)"

  local alice_choice_ctr=${alice_ctrs[0]}
  local bob_choice_ctr=${bob_ctrs[0]}

  docker cp "$alice_choice_ctr":/home/kuscia/var/certs/domain.crt alice.domain.crt
  for ctr in "${bob_ctrs[@]}"; do
    docker cp alice.domain.crt "$ctr":/home/kuscia/var/certs/alice.domain.crt
    rm_iptables "$ctr"
  done

  docker cp "$bob_choice_ctr":/home/kuscia/var/certs/domain.crt bob.domain.crt
  for ctr in "${alice_ctrs[@]}"; do
    docker cp bob.domain.crt "$ctr":/home/kuscia/var/certs/bob.domain.crt
    rm_iptables "$ctr"
  done

  docker exec -it "${alice_choice_ctr}" scripts/deploy/add_domain.sh bob p2p
  docker exec -it "${bob_choice_ctr}" scripts/deploy/add_domain.sh alice p2p
  docker exec -it "${alice_choice_ctr}" scripts/deploy/join_to_host.sh alice bob https://"$hostname":24869
  docker exec -it "${bob_choice_ctr}" scripts/deploy/join_to_host.sh bob alice https://"$hostname":14869
  # reverse tunnel
  docker exec -it "${alice_choice_ctr}" kubectl patch cdr alice-bob --type=merge -p '{"spec":{"transit":{"transitMethod":"REVERSE-TUNNEL"}}}'
  docker exec -it "${bob_choice_ctr}" kubectl patch cdr alice-bob --type=merge -p '{"spec":{"transit":{"transitMethod":"REVERSE-TUNNEL"}}}'
  # create demo data
  create_domaindata_alice_table "${alice_choice_ctr}" alice
  create_domaindata_bob_table "${bob_choice_ctr}" bob
  create_domaindatagrant_alice2bob "${alice_choice_ctr}"
  create_domaindatagrant_bob2alice "${bob_choice_ctr}"
  # create secretflow app image
  create_secretflow_app_image "${alice_choice_ctr}"
  create_secretflow_app_image "${bob_choice_ctr}"
}

function rm_iptables() {
  local ctr
  local pid
  local name

  ctr=$1
  pid=$(docker inspect --format '{{.State.Pid}}' "$ctr")
  name=$(docker inspect --format '{{.Name}}' "$ctr" | cut -b 2- | cut -d '.' -f1,2 | sed 's/_/-/g')
  nsenter -t "$pid" -n iptables -L -n -v
  nsenter -t "$pid" -n iptables -F INPUT
  nsenter -t "$pid" -n iptables -F OUTPUT

  # nsenter -t $pid --uts hostname $name
}

function create_network() {
  local hostname
  hostname=$(hostname -I | awk '{print $1}')

  network_name="kuscia-swarm-exchange"
  exists=$(docker network ls | grep -c $network_name)
  if [ "$exists" != "1" ]; then
    docker swarm init --advertise-addr "$hostname"
    docker network create -d overlay --subnet 12.0.0.0/8 --attachable $network_name
  else
    echo "network $network_name exists!"
  fi
}

function create_kuscia_yaml() {
  local hostname
  hostname=$(hostname -I | awk '{print $1}')

  if [ ! -d alice ]; then
    mkdir alice
  fi
  docker run -it --rm "${KUSCIA_IMAGE}" kuscia init --mode autonomy --domain "alice" --runtime "runp" --log-level "DEBUG" --datastore-endpoint "mysql://root:password@tcp($hostname:13307)/kine" > alice/kuscia.yaml 2>&1 || cat alice/kuscia.yaml

  if [ ! -d bob ]; then
    mkdir bob
  fi
  docker run -it --rm "${KUSCIA_IMAGE}" kuscia init --mode autonomy --domain "bob" --runtime "runp" --log-level "DEBUG" --datastore-endpoint "mysql://root:password@tcp($hostname:13308)/kine" > bob/kuscia.yaml 2>&1 || cat bob/kuscia.yaml
}

function create_load() {
  script_dir=$(realpath "$(dirname "$0")")
  cat << EOF > kuscia-autonomy.yaml
  version: '3.8'

  services:
    kuscia-autonomy-alice:
      image: $IMAGE
      command:
        - bin/kuscia
        - start
        - -c
        - etc/conf/kuscia.yaml
      environment:
        NAMESPACE: alice
        PATH: '/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:home/kuscia/tmp/bin:/home/kuscia/bin:/bin/aux'
      volumes:
        - /tmp:/tmp
        - $script_dir/alice/kuscia.yaml:/home/kuscia/etc/conf/kuscia.yaml
      ports:
        - "14869:1080/tcp"
      networks:
        - kuscia-swarm-exchange
      depends_on:
        - mysql-alice
      deploy:
        replicas: 3

    kuscia-autonomy-bob:
      image: $IMAGE
      command:
        - bin/kuscia
        - start
        - -c
        - etc/conf/kuscia.yaml
      environment:
        NAMESPACE: bob
        PATH: '/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:home/kuscia/tmp/bin:/home/kuscia/bin:/bin/aux'
      volumes:
        - /tmp:/tmp
        - $script_dir/bob/kuscia.yaml:/home/kuscia/etc/conf/kuscia.yaml
      ports:
        - "24869:1080/tcp"
      networks:
      networks:
        - kuscia-swarm-exchange
      depends_on:
        - mysql-bob
      deploy:
        replicas: 1

    mysql-alice:
      image: mysql:8.0
      environment:
        MYSQL_ROOT_PASSWORD: password
        MYSQL_DATABASE: kine
        MYSQL_USER: user
        MYSQL_PASSWORD: password
      ports:
        - "13307:3306"
      networks:
        - kuscia-swarm-exchange

    mysql-bob:
      image: mysql:8.0
      environment:
        MYSQL_ROOT_PASSWORD: password
        MYSQL_DATABASE: kine
        MYSQL_USER: user
        MYSQL_PASSWORD: password
      ports:
        - "13308:3306"
      networks:
        - kuscia-swarm-exchange

  networks:
    kuscia-swarm-exchange:
      name: kuscia-swarm-exchange
      external: true
EOF
  docker stack deploy -c kuscia-autonomy.yaml kuscia-autonomy
}

function clean_replica() {
  docker stack rm kuscia-autonomy
}

function run_replica() {
  create_network
  clean_replica
  sleep 10
  create_kuscia_yaml
  create_load
  sleep 60
  build_replica_conn
}

run_replica