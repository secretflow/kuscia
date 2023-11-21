import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns

import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
period = "5"
lab_id = "3"
metric_num = "20"
linewidth=5
dpi=100
markersize=12
fontsize=35
ticksize=35
width=13
height=7

label = ["vsz", "rss", "usage"]
color=["#ff9999", "#ffd480", "#09b538", "#CC3300", "dodgerblue", ]
marker = ['o','v','*','D','H','+','x','X','d','|','_']
linestyle=['-','--','-.','--','dotted']

fig = plt.figure(figsize=(width,height),dpi=dpi)
fig.subplots_adjust(bottom=0.15,left=0.15)

xdata = []
vsz_data = []
rss_data = []
mem_data = []
send_data = []
recv_data = []
net_data = []
cpu_data = []
f = open("/home/kuscia/E2EMon-1.0/exp/exp" + lab_id + "/sysdata")
timestamp = 0
while True:
    line = f.readline()
    if line == "":
        break
    timestamp += int(period)
    xdata.append(timestamp)
    lines = line.split(";")
    vsz_data.append(float(lines[0])/1024)
    rss_data.append(float(lines[1])/1024)
    mem_data.append(float(lines[2]))
    send_data.append(float(lines[3])/(1024*1024))
    recv_data.append(float(lines[4])/(1024*1024))
    net_data.append((float(lines[3])+float(lines[4]))/(1024*1024*2))
    cpu_data.append(float(lines[5]))

# plt.plot(xdata, vsz_data, color=color[0] ,linestyle=linestyle[0], marker = marker[0], markersize=markersize, linewidth=linewidth, label=label[0])
plt.plot(xdata, rss_data, color=color[1] ,linestyle=linestyle[1], marker = marker[1], markersize=markersize, linewidth=linewidth, label=label[1])
# plt.ylim(0, 1.02)
plt.xlabel("Time (s)", fontsize=fontsize)
plt.ylabel("Memory Usage (MB)", fontsize=fontsize)
# plt.legend(bbox_to_anchor=(0.1, 1.02, 1, 0.2), fontsize=fontsize-2, ncol=5)
plt.legend(loc="best", fontsize=fontsize, ncol=2, frameon=False)
plt.tick_params(labelsize=ticksize)
plt.savefig('/home/kuscia/E2EMon-1.0/exp/exp' + lab_id + '/mem-' + period + "-" + metric_num + '.pdf')
plt.savefig('/home/kuscia/E2EMon-1.0/exp/exp' + lab_id + '/mem-' + period + "-" + metric_num + '.png')
fig = plt.figure(figsize=(width,height),dpi=dpi)
fig.subplots_adjust(bottom=0.15,left=0.15)

label = ["Avg", "Send", "Recv"]
plt.plot(xdata, net_data, color=color[0], linestyle=linestyle[0], marker = marker[0], markersize=markersize, linewidth=linewidth, label=label[0])
plt.plot(xdata, send_data, color=color[1], linestyle=linestyle[1], marker = marker[1], markersize=markersize, linewidth=linewidth, label=label[1])
plt.plot(xdata, recv_data, color=color[2], linestyle=linestyle[2], marker = marker[2], markersize=markersize, linewidth=linewidth, label=label[2])
plt.xlabel("Time (s)", fontsize=fontsize)
plt.ylabel("NetIO (MB)", fontsize=fontsize)
plt.legend(loc="best", fontsize=fontsize, ncol=2, frameon=False)
plt.tick_params(labelsize=ticksize)
plt.savefig('/home/kuscia/E2EMon-1.0/exp/exp' + lab_id + '/net-' + period + "-" + metric_num  + '.pdf')
plt.savefig('/home/kuscia/E2EMon-1.0/exp/exp' + lab_id + '/net-' + period + "-" + metric_num  + '.png')


label = ["Mem", "CPU"]
fig = plt.figure(figsize=(width,height),dpi=dpi)
fig.subplots_adjust(bottom=0.15,left=0.15)
plt.plot(xdata, mem_data, color=color[0], linestyle=linestyle[0], marker = marker[0], markersize=markersize, linewidth=linewidth, label=label[0])
plt.plot(xdata, cpu_data, color=color[1], linestyle=linestyle[1], marker = marker[1], markersize=markersize, linewidth=linewidth, label=label[1])
plt.xlabel("Time (s)", fontsize=fontsize)
plt.ylabel("Usage (%)", fontsize=fontsize)
plt.legend(loc="best", fontsize=fontsize, ncol=2, frameon=False)
plt.tick_params(labelsize=ticksize)
plt.savefig('/home/kuscia/E2EMon-1.0/exp/exp' + lab_id + '/usage-' + period + "-" + metric_num + '.pdf')
plt.savefig('/home/kuscia/E2EMon-1.0/exp/exp' + lab_id + '/usage-' + period + "-" + metric_num + '.png')
