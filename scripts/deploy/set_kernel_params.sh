#!/bin/bash
#
# Copyright 2024 Ant Group Co., Ltd.
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


# maximum size of the SYN backlog queue in a TCP connection
echo 2048 > /proc/sys/net/ipv4/tcp_max_syn_backlog

# wait accept queue size
echo 2048 > /proc/sys/net/core/somaxconn

# cp retry times, 5 --> 25.6s-51.2s, 15 --> 924.6s-1044.6s
echo 5 > /proc/sys/net/ipv4/tcp_retries2

# controls the behavior of TCP slow start when a connection remains idle for a certain period of time.
echo 0 > /proc/sys/net/ipv4/tcp_slow_start_after_idle

# client reuse TIME_WAIT socket. 1 --> enable
echo 1 > /proc/sys/net/ipv4/tcp_tw_reuse

# max open files
echo 102400 > /proc/sys/fs/file-max