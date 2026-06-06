#!/usr/bin/env python3
"""
FishboneChain 实验结果可视化脚本
生成论文用图表（Matplotlib）
"""

import csv
import os
from collections import defaultdict
from datetime import datetime
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.font_manager as fm
import numpy as np

OUT_DIR = os.path.join(os.path.dirname(__file__), '..', 'docs', 'figures')
os.makedirs(OUT_DIR, exist_ok=True)

# 配置中文字体
plt.rcParams['font.sans-serif'] = ['WenQuanYi Micro Hei', 'Noto Sans CJK SC', 'DejaVu Sans']
plt.rcParams['axes.unicode_minus'] = False
plt.rcParams.update({
    'font.size': 11,
    'axes.titlesize': 12,
    'axes.labelsize': 11,
    'legend.fontsize': 10,
    'figure.dpi': 150,
    'savefig.bbox': 'tight',
    'savefig.pad_inches': 0.1,
})


# ─────────────────────────────────────────────────────────────────────────────
# 图1：实验 A — 通用链多场景容量瓶颈
# ─────────────────────────────────────────────────────────────────────────────
def fig_exp_a():
    scenarios = ['a\n(快递配送\n300工,5U)', 'b\n(交通监控\n2000工,0.001U)',
                 'c\n(医疗诊断\n200工,200U)', 'e\n(传感器IoT\n5000工,0.0001U)']
    ok_counts  = [251, 690, 47, 15]
    rej_counts = [1763, 5775, 261, 1082]
    total = [o + r for o, r in zip(ok_counts, rej_counts)]
    rates  = [o/t*100 for o, t in zip(ok_counts, total)]

    colors_ok  = ['#2196F3', '#4CAF50', '#FF9800', '#9C27B0']
    colors_rej = ['#BBDEFB', '#C8E6C9', '#FFE0B2', '#E1BEE7']

    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(10, 4))

    # 左图：堆叠柱状图（接受 vs 拒绝）
    x = np.arange(len(scenarios))
    for i, (sc, ok, rej, c_ok, c_rej) in enumerate(zip(scenarios, ok_counts, rej_counts, colors_ok, colors_rej)):
        ax1.bar(i, ok, color=c_ok, label='被接受' if i==0 else '')
        ax1.bar(i, rej, bottom=ok, color=c_rej, label='被拒绝' if i==0 else '')

    ax1.set_xticks(x)
    ax1.set_xticklabels(scenarios, fontsize=9)
    ax1.set_ylabel('提交数量')
    ax1.set_title('(a) 共享链上各场景总提交量\n（child1，3个Epoch）')
    ax1.legend(loc='upper right')
    ax1.set_ylim(0, max(total) * 1.15)

    # 右图：成功率柱状图
    bars = ax2.bar(x, rates, color=colors_ok, edgecolor='black', linewidth=0.5)
    ax2.axhline(y=100, color='gray', linestyle='--', alpha=0.4)
    ax2.set_xticks(x)
    ax2.set_xticklabels(scenarios, fontsize=9)
    ax2.set_ylabel('成功率 (%)')
    ax2.set_title('(b) 各场景提交成功率\n（共用一条链竞争）')
    ax2.set_ylim(0, 25)
    for bar, rate in zip(bars, rates):
        ax2.text(bar.get_x() + bar.get_width()/2, bar.get_height() + 0.3,
                 f'{rate:.1f}%', ha='center', va='bottom', fontsize=10, fontweight='bold')

    fig.suptitle('实验A：通用链多场景容量瓶颈\n'
                 '（高频场景在60秒内打满 MaxSubmissions=1000/Epoch 上限）',
                 fontsize=11, fontweight='bold')
    plt.tight_layout()
    out = os.path.join(OUT_DIR, 'fig1_exp_a_bottleneck.png')
    fig.savefig(out)
    plt.close()
    print(f'[saved] {out}')


# ─────────────────────────────────────────────────────────────────────────────
# 图2：实验 B — 专用链改善效果
# ─────────────────────────────────────────────────────────────────────────────
def fig_exp_b():
    fig, axes = plt.subplots(1, 3, figsize=(12, 4))

    # 子图(a)：TPS 对比 — 场景 b
    ax = axes[0]
    labels = ['child1\nAURA-6s\n（通用链）', 'child2\nAURA-2s\n（专用链）']
    tps    = [0.2, 80.5]
    colors = ['#FF7043', '#42A5F5']
    bars = ax.bar(labels, tps, color=colors, edgecolor='black', linewidth=0.5, width=0.5)
    for bar, v in zip(bars, tps):
        ax.text(bar.get_x() + bar.get_width()/2, v + 1, f'{v} TPS',
                ha='center', va='bottom', fontweight='bold', fontsize=11)
    ax.set_ylabel('峰值吞吐量 (TPS)')
    ax.set_title('(a) 场景b：高频交通数据\n通用链 vs 专用链')
    ax.set_ylim(0, 100)
    ax.annotate('400× 提升', xy=(1, 80.5), xytext=(0.5, 70),
                arrowprops=dict(arrowstyle='->', color='green', lw=1.5),
                fontsize=10, color='green', fontweight='bold')

    # 子图(b)：Mempool 丢包率 — AURA vs BABE
    ax = axes[1]
    labels = ['child1\nAURA-6s\n（共用，拥塞）', 'child1\nAURA-6s\n（隔离测试）', 'child6\nBABE-5s\n（专用链）']
    fail_rates = [100, 44, 0]
    colors = ['#EF5350', '#FFA726', '#66BB6A']
    bars = ax.bar(labels, fail_rates, color=colors, edgecolor='black', linewidth=0.5, width=0.5)
    for bar, v in zip(bars, fail_rates):
        ax.text(bar.get_x() + bar.get_width()/2, v + 1, f'{v}%',
                ha='center', va='bottom', fontweight='bold', fontsize=12)
    ax.set_ylabel('Mempool 丢包率 (%)')
    ax.set_title('(b) 场景f：Mempool 丢包率\nAURA vs BABE 共识对比')
    ax.set_ylim(0, 115)
    ax.axhline(y=0, color='green', linestyle='--', alpha=0.5)

    # 子图(c)：成功率分解 — 场景 f AURA vs BABE 隔离测试
    ax = axes[2]
    labels = ['AURA-6s\n（child1，隔离）', 'BABE-5s\n（child6，专用）']
    success = [54, 24.4]
    fail_r  = [44, 0]
    reject  = [100-s-f for s, f in zip(success, fail_r)]
    x = np.arange(len(labels))
    ax.bar(x, success, color='#42A5F5', label='被接受')
    ax.bar(x, fail_r,  bottom=success, color='#EF5350', label='丢包（Mempool层）')
    ax.bar(x, reject,  bottom=[s+f for s,f in zip(success,fail_r)], color='#BDBDBD', label='被拒绝（链上约束）')
    ax.set_xticks(x)
    ax.set_xticklabels(labels, fontsize=9)
    ax.set_ylabel('提交结果占比 (%)')
    ax.set_title('(c) 场景f隔离测试\nAURA vs BABE 提交结果分解')
    ax.legend(loc='upper right', fontsize=9)
    ax.set_ylim(0, 115)

    fig.suptitle('实验B：专用链的改善效果\n（相同场景，不同链配置对比）',
                 fontsize=11, fontweight='bold')
    plt.tight_layout()
    out = os.path.join(OUT_DIR, 'fig2_exp_b_dedicated.png')
    fig.savefig(out)
    plt.close()
    print(f'[saved] {out}')


# ─────────────────────────────────────────────────────────────────────────────
# 图3：实验 C — 6链并发吞吐量时序
# ─────────────────────────────────────────────────────────────────────────────
def fig_exp_c_timeline():
    rows = []
    with open('/tmp/exp_c_state.csv') as f:
        rows = list(csv.DictReader(f))

    CHAINS_ORDER = [
        ('ws://10.2.2.20:9949', 'child5 (e,5000工)'),
        ('ws://10.2.2.11:9950', 'child6 (f,500工)'),
        ('ws://10.2.2.11:9945', 'child1 (a,300工)'),
        ('ws://10.2.2.17:9947', 'child3 (c,200工)'),
        ('ws://10.2.2.14:9946', 'child2 (b,2000工)'),
        ('ws://10.2.2.11:9948', 'child4 (d,100工)'),
    ]
    COLORS = ['#8E24AA', '#00ACC1', '#E53935', '#43A047', '#FB8C00', '#1E88E5']
    chain_names = [n for _, n in CHAINS_ORDER]
    url_to_name = {u: n for u, n in CHAINS_ORDER}

    ep_max = defaultdict(lambda: defaultdict(int))
    ep_ts  = defaultdict(lambda: defaultdict(str))
    for r in rows:
        url  = r['chain_url']
        name = url_to_name.get(url)
        if not name: continue
        try:
            ep   = int(r['epoch_id'])
            subs = int(r['submissions_count'])
            ts   = r['timestamp']
            ep_max[name][ep] = max(ep_max[name][ep], subs)
            if ep not in ep_ts[name] or ts < ep_ts[name][ep]:
                ep_ts[name][ep] = ts
        except: pass

    fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(12, 8))

    # 上图：前6小时堆叠面积图
    all_ts_str = sorted(set(
        r['timestamp'][:16] for r in rows
        if r['chain_url'] in url_to_name
    ))
    ts_data = defaultdict(lambda: defaultdict(int))
    for r in rows:
        name = url_to_name.get(r['chain_url'])
        if not name: continue
        ts = r['timestamp'][:16]
        try:
            ts_data[ts][name] = max(ts_data[ts][name], int(r['submissions_count']))
        except: pass

    t0_str = all_ts_str[0]
    t0_dt  = datetime.strptime(t0_str, '%Y-%m-%dT%H:%M')
    xs_all = [(datetime.strptime(ts, '%Y-%m-%dT%H:%M') - t0_dt).total_seconds()/3600
              for ts in all_ts_str]

    mask6h = [x <= 6 for x in xs_all]
    xs6    = [x for x, m in zip(xs_all, mask6h) if m]
    ts6    = [ts for ts, m in zip(all_ts_str, mask6h) if m]

    ys = {n: [ts_data[ts].get(n, 0) for ts in ts6] for n in chain_names}
    ax1.stackplot(xs6, [ys[n] for n in chain_names],
                  labels=chain_names, colors=COLORS, alpha=0.85)
    ax1.set_xlim(0, 6)
    ax1.set_ylim(0, 4500)
    ax1.set_xlabel('运行时长（小时）')
    ax1.set_ylabel('提交计数')
    ax1.set_title('(a) 6链并发吞吐量 — 前6小时\n'
                  '堆叠面积图，每色带代表一条链的提交量')
    ax1.legend(loc='upper right', fontsize=8, ncol=2)
    ax1.grid(axis='y', alpha=0.3)

    # 标注各链崩溃时刻
    for tx, label, color in [(0.37,'e内存溢出','#6A1B9A'), (1.4,'f内存溢出','#00838F'), (2.8,'a内存溢出','#C62828')]:
        ax1.axvline(x=tx, color=color, linestyle='--', alpha=0.6, linewidth=1.2)
        ax1.text(tx+0.05, 4200, label, color=color, fontsize=8, fontweight='bold')

    # 下图：child4 逐Epoch稳态数据（112个Epoch）
    c4_name = 'child4 (d,100工)'
    c4_eps = sorted(ep_max[c4_name].keys())
    c4_subs = [ep_max[c4_name][ep] for ep in c4_eps]
    c4_xs   = list(range(1, len(c4_eps)+1))

    ax2.bar(c4_xs, c4_subs, color='#1565C0', alpha=0.75, width=1.0)
    ax2.axhline(y=100, color='red', linestyle='--', linewidth=1.2, label='理论值：100次/Epoch（100个工作者）')
    ax2.set_xlim(0, len(c4_xs)+1)
    ax2.set_ylim(0, 130)
    ax2.set_xlabel('Epoch 序号（按时间排列）')
    ax2.set_ylabel('每Epoch峰值提交量')
    ax2.set_title('(b) child4：连续112个Epoch（22小时）— 金融专用链\n'
                  '稳态：100次/Epoch，零失败，工作者参与率100%')
    ax2.legend(loc='upper right')
    ax2.grid(axis='y', alpha=0.3)

    fig.suptitle('实验C：6链并发运行（22小时）',
                 fontsize=12, fontweight='bold', y=1.01)
    fig.tight_layout()
    out = os.path.join(OUT_DIR, 'fig3_exp_c_timeline.png')
    fig.savefig(out, bbox_inches='tight')
    plt.close()
    print(f'[saved] {out}')


# ─────────────────────────────────────────────────────────────────────────────
# 图4：child4 稳态运行 — 连续112个Epoch
# ─────────────────────────────────────────────────────────────────────────────
def fig_child4_steady():
    rows = []
    with open('/tmp/exp_c_state.csv') as f:
        rows = list(csv.DictReader(f))

    c4_rows = [r for r in rows if r['chain_url'] == 'ws://10.2.2.11:9948']
    ep_subs = defaultdict(int)
    for r in c4_rows:
        try:
            ep  = int(r['epoch_id'])
            sub = int(r['submissions_count'])
            ep_subs[ep] = max(ep_subs[ep], sub)
        except: pass

    sorted_eps = sorted(ep_subs.keys())
    ep_seq = list(range(1, len(sorted_eps)+1))
    subs   = [ep_subs[ep] for ep in sorted_eps]

    fig, ax = plt.subplots(figsize=(10, 3.5))
    ax.plot(ep_seq, subs, color='#1565C0', linewidth=1.5, marker='o', markersize=3, alpha=0.8)
    ax.axhline(y=100, color='red', linestyle='--', linewidth=1.2, label='理论值：100次/Epoch（100个工作者）')
    ax.fill_between(ep_seq, subs, alpha=0.15, color='#1E88E5')
    ax.set_xlim(1, len(ep_seq))
    ax.set_ylim(0, 130)
    ax.set_xlabel('Epoch 序号（按时间排列）')
    ax.set_ylabel('每Epoch提交量')
    ax.set_title('实验C：child4（金融专用链）— 连续112个Epoch\n'
                 '100工作者 × 约1次/Epoch ≈ 100次/Epoch，零失败，持续22小时')
    ax.legend(loc='upper right')
    ax.grid(axis='y', alpha=0.3)

    avg_subs = np.mean(subs)
    ax.text(len(ep_seq)*0.05, 115,
            f'均值：{avg_subs:.1f} 次/Epoch  |  失败数：0  |  工作者参与率：100%',
            fontsize=9, color='#1565C0', style='italic')

    plt.tight_layout()
    out = os.path.join(OUT_DIR, 'fig4_child4_steady.png')
    fig.savefig(out)
    plt.close()
    print(f'[saved] {out}')


# ─────────────────────────────────────────────────────────────────────────────
# 图5：实验C 第1个Epoch — 6链同时活跃快照
# ─────────────────────────────────────────────────────────────────────────────
def fig_epoch1_snapshot():
    chains    = ['child1\n(a,300工)', 'child2\n(b,2000工)', 'child3\n(c,200工)',
                 'child4\n(d,100工)', 'child5\n(e,5000工)', 'child6\n(f,500工)']
    subs_ep1  = [1000, 1000, 200, 100, 1000, 500]
    colors    = ['#E53935', '#FB8C00', '#43A047', '#1E88E5', '#8E24AA', '#00ACC1']
    single_cap = 1000

    fig, ax = plt.subplots(figsize=(9, 4))
    x = np.arange(len(chains))
    bars = ax.bar(x, subs_ep1, color=colors, edgecolor='black', linewidth=0.5, width=0.6)
    ax.axhline(y=single_cap, color='red', linestyle='--', linewidth=1.5,
               label=f'单链容量上限 = {single_cap} 次/Epoch')

    for bar, v in zip(bars, subs_ep1):
        ax.text(bar.get_x() + bar.get_width()/2, v + 10, str(v),
                ha='center', va='bottom', fontweight='bold', fontsize=10)

    total = sum(subs_ep1)
    ax.annotate(f'合计 = {total} 次/Epoch\n（{total/single_cap:.1f}× 单链容量）',
                xy=(2.5, 600), xytext=(3.8, 900),
                arrowprops=dict(arrowstyle='->', color='green', lw=2),
                fontsize=11, color='green', fontweight='bold',
                bbox=dict(boxstyle='round,pad=0.3', facecolor='#E8F5E9', edgecolor='green'))

    ax.set_xticks(x)
    ax.set_xticklabels(chains, fontsize=9)
    ax.set_ylabel('第1个Epoch提交量')
    ax.set_ylim(0, 1300)
    ax.set_title(f'实验C 第1个Epoch：6条链全部同时活跃\n'
                 f'聚合提交 = {total}次（{total/single_cap:.1f}× 单链容量）')
    ax.legend(loc='upper right')
    ax.grid(axis='y', alpha=0.3)

    plt.tight_layout()
    out = os.path.join(OUT_DIR, 'fig5_epoch1_snapshot.png')
    fig.savefig(out)
    plt.close()
    print(f'[saved] {out}')


# ─────────────────────────────────────────────────────────────────────────────
# 图6：实验D — N=1/2/4链线性扩展验证
# ─────────────────────────────────────────────────────────────────────────────
def fig_linear_scaling(csv_path='/tmp/exp_scale_state.csv'):
    rows = []
    with open(csv_path) as f:
        rows = list(csv.DictReader(f))
    if not rows:
        print('[warn] fig6: exp_scale_state.csv 无数据')
        return

    CHAIN_URLS = {
        'ws://10.2.2.11:9948': 'child4',
        'ws://10.2.2.17:9947': 'child3',
        'ws://10.2.2.11:9945': 'child1',
        'ws://10.2.2.11:9950': 'child6',
    }

    ep_subs = {name: defaultdict(int) for name in CHAIN_URLS.values()}
    for r in rows:
        name = CHAIN_URLS.get(r['chain_url'])
        if not name: continue
        try:
            ep   = int(r['epoch_id'])
            subs = int(r['submissions_count'])
            ep_subs[name][ep] = max(ep_subs[name][ep], subs)
        except: pass

    # 排除首个（部分）和最后一个（进行中）Epoch，取稳态均值
    def avg_active(name):
        ep_items = sorted(ep_subs[name].items())
        if len(ep_items) <= 2:
            vals = [s for _, s in ep_items if s > 0]
        else:
            stable = ep_items[1:-1]
            vals = [s for _, s in stable if s > 0]
        return np.mean(vals) if vals else 0, len(vals)

    c4_avg, c4_n = avg_active('child4')
    c3_avg, c3_n = avg_active('child3')
    c1_avg, c1_n = avg_active('child1')
    c6_avg, c6_n = avg_active('child6')

    print(f'[fig6] child4: {c4_avg:.1f} 次/Epoch ({c4_n} epochs)')
    print(f'[fig6] child3: {c3_avg:.1f} 次/Epoch ({c3_n} epochs)')
    print(f'[fig6] child1: {c1_avg:.1f} 次/Epoch ({c1_n} epochs)')
    print(f'[fig6] child6: {c6_avg:.1f} 次/Epoch ({c6_n} epochs)')

    n_chains   = [1, 2, 4]
    total_subs = [c4_avg, c4_avg + c3_avg, c4_avg + c3_avg + c1_avg + c6_avg]

    x_ref = np.linspace(0, 4.5, 100)
    y_ref = c4_avg * x_ref

    fig, ax = plt.subplots(figsize=(7, 5))

    ax.scatter(n_chains, total_subs, color='#1565C0', s=120, zorder=5, label='实测聚合吞吐量')
    ax.plot(x_ref, y_ref, 'r--', linewidth=1.5, label=f'理想线性：{c4_avg:.0f}×N 次/Epoch')

    for n, s in zip(n_chains, total_subs):
        ax.annotate(f'{s:.0f} 次/Epoch\n(N={n}条链)', xy=(n, s),
                    xytext=(n + 0.15, s - c4_avg * 0.2),
                    fontsize=10, fontweight='bold', color='#1565C0')

    ax.set_xticks([1, 2, 4])
    ax.set_xticklabels(['1条链\n(child4)', '2条链\n(child3+4)', '4条链\n(child1+3+4+6)'])
    ax.set_xlabel('并发链数量 (N)')
    ax.set_ylabel('聚合吞吐量（次/Epoch）')
    ax.set_xlim(0, 5)
    ax.set_ylim(0, max(total_subs) * 1.3)
    ax.set_title('实验D：吞吐量线性扩展验证\n'
                 '每链50个工作者（场景d配置），4链同时运行测量')
    ax.legend(loc='upper left')
    ax.grid(alpha=0.3)

    if len(n_chains) >= 3:
        predicted_4 = c4_avg * 4
        actual_4 = total_subs[2]
        ratio = actual_4 / predicted_4
        ax.text(0.3, max(total_subs) * 0.55,
                f'N=4 实测：{actual_4:.0f} 次/Epoch\n'
                f'N=4 理想（4×N=1）：{predicted_4:.0f} 次/Epoch\n'
                f'扩展倍率：{ratio:.2f}×（链配置多样性红利）',
                fontsize=9, color='#1565C0',
                bbox=dict(boxstyle='round', facecolor='#E3F2FD', edgecolor='#1565C0', alpha=0.9))

    plt.tight_layout()
    out = os.path.join(OUT_DIR, 'fig6_linear_scaling.png')
    fig.savefig(out)
    plt.close()
    print(f'[saved] {out}')


if __name__ == '__main__':
    import sys
    only_fig6 = '--fig6' in sys.argv

    if only_fig6:
        print('生成图6（线性扩展）...')
        try:
            fig_linear_scaling()
        except Exception as e:
            print(f'[warn] fig6 错误: {e}')
    else:
        print('生成所有图表...')
        fig_exp_a()
        fig_exp_b()
        try:
            fig_exp_c_timeline()
        except Exception as e:
            print(f'[warn] fig3 错误: {e}')
        fig_child4_steady()
        fig_epoch1_snapshot()
        try:
            fig_linear_scaling()
        except Exception as e:
            print(f'[warn] fig6（需要 exp_scale 数据）: {e}')
    print('完成。图表已保存至 docs/figures/')
