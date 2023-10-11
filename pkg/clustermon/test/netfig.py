import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns

linewidth=5
dpi=100
markersize=12
fontsize=35
ticksize=35
width=18
height=7

label = []
color=["#ff9999", "#ffd480", "#09b538", "#CC3300", "dodgerblue", ]
marker = ['o','v','*','D','H','+','x','X','d','|','_']
linestyle=['-','--','-.','--','dotted']


xdata = []
ydata = []
f = open("./monitordata/data")
timestamp = 0
metrics = dict()
while True:
    line = f.readline()
    if line == "":
        break
    timestamp += 3
    xdata.append(timestamp)
    lines = line.split(";")
    for x in lines:
        kv = x.split(":")
        if len(kv) >= 2:
            key = kv[0]
            val = float(kv[1])
        
        if key not in metrics:
            metrics[key] = list()
        metrics[key].append(val)

keys = ["cluster.alice-to-bob-HTTP.update_attempt","cluster.alice-to-bob-HTTP.upstream_cx_total", "bytesSentAvg"]
for i in range(len(keys)):
    key = keys[i]
    fig = plt.figure(figsize=(width,height),dpi=dpi)
    fig.subplots_adjust(bottom=0.15,left=0.15)
    print(xdata, metrics[key])
    plt.plot(xdata, metrics[key], color=color[0] ,linestyle=linestyle[0], marker = marker[0], markersize=markersize, linewidth=linewidth, label=key)
    # plt.ylim(0, 1.02)
    plt.xlabel("Time (Rounds)", fontsize=fontsize)
    plt.ylabel(key, fontsize=fontsize)
    # plt.legend(bbox_to_anchor=(0.1, 1.02, 1, 0.2), fontsize=fontsize-2, ncol=5)
    plt.legend(loc="best", fontsize=fontsize, ncol=2, frameon=False)
    plt.tick_params(labelsize=ticksize)
    plt.savefig("./exp/exp1"+key+".pdf")

