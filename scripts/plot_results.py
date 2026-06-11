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
def fig_linear_scaling(csv_path=None):
    if csv_path is None:
        csv_path = os.path.join(os.path.dirname(__file__), '..', 'docs', 'figures', 'data',
                                'exp_scale_state.csv')
        if not os.path.exists(csv_path):
            csv_path = '/tmp/exp_scale_state.csv'

    rows = []
    with open(csv_path) as f:
        rows = list(csv.DictReader(f))
    if not rows:
        print('[warn] fig6: 无数据')
        return

    # N=1: child4; N=2: child4+child1; N=3: child4+child1+child6
    CHAIN_URLS = {
        'ws://10.2.2.11:9948': 'child4',
        'ws://10.2.2.11:9945': 'child1',
        'ws://10.2.2.11:9950': 'child6',
    }
    CHAIN_COLORS = {'child4': '#1565C0', 'child1': '#2E7D32', 'child6': '#E65100'}

    # 每条链 per-epoch 峰值提交量
    ep_subs = {name: defaultdict(int) for name in CHAIN_URLS.values()}
    for r in rows:
        name = CHAIN_URLS.get(r['chain_url'])
        if not name: continue
        try:
            ep   = int(r['epoch_id'])
            subs = int(r['submissions_count'])
            ep_subs[name][ep] = max(ep_subs[name][ep], subs)
        except: pass

    def stable_vals(name):
        items = sorted(ep_subs[name].items())
        if len(items) <= 2:
            return [s for _, s in items if s > 0]
        return [s for _, s in items[1:-1] if s > 0]  # 去掉首尾不完整 epoch

    c4_vals = stable_vals('child4')
    c1_vals = stable_vals('child1')
    c6_vals = stable_vals('child6')

    for name, vals in [('child4', c4_vals), ('child1', c1_vals), ('child6', c6_vals)]:
        print(f'[fig6] {name}: mean={np.mean(vals):.1f}  std={np.std(vals):.1f}  n={len(vals)}')

    # 对齐 epoch 数（取最少的那个）
    n_ep = min(len(c4_vals), len(c1_vals), len(c6_vals))
    c4 = np.array(c4_vals[:n_ep])
    c1 = np.array(c1_vals[:n_ep])
    c6 = np.array(c6_vals[:n_ep])

    # 三个 N 配置的每 epoch 聚合值
    n1_vals = c4                   # N=1: child4 alone
    n2_vals = c4 + c1              # N=2: child4 + child1
    n3_vals = c4 + c1 + c6        # N=3: all three

    configs = [
        (1, n1_vals, 'child4',           '#1565C0'),
        (2, n2_vals, 'child4 + child1',  '#2E7D32'),
        (3, n3_vals, 'child4+child1+child6', '#E65100'),
    ]

    fig, ax = plt.subplots(figsize=(8, 5.5))

    # 散点：每个 epoch 一个点（加随机 x 抖动以免重叠）
    rng = np.random.default_rng(42)
    all_means = []
    for n, vals, label, color in configs:
        jitter = rng.uniform(-0.06, 0.06, size=len(vals))
        ax.scatter(np.full(len(vals), n) + jitter, vals,
                   color=color, alpha=0.45, s=40, zorder=4)
        mean_v = np.mean(vals)
        std_v  = np.std(vals)
        ax.errorbar(n, mean_v, yerr=std_v, fmt='o', color=color,
                    markersize=10, linewidth=2, capsize=6, zorder=5,
                    label=f'N={n}  ({label})\n均值={mean_v:.0f}，σ={std_v:.1f}')
        all_means.append(mean_v)

    # 理想线性参考线（过原点，斜率 = N=1 均值）
    baseline = all_means[0]
    x_ref = np.linspace(0.5, 3.5, 100)
    y_ref = baseline * x_ref
    ax.plot(x_ref, y_ref, 'k--', linewidth=1.4, alpha=0.5, label=f'理想线性 ({baseline:.0f}×N)')

    # 线性回归 + R²
    from numpy.polynomial import polynomial as P
    ns = np.array([1, 2, 3], dtype=float)
    coeffs = np.polyfit(ns, all_means, 1)
    y_fit = np.polyval(coeffs, ns)
    ss_res = np.sum((np.array(all_means) - y_fit) ** 2)
    ss_tot = np.sum((np.array(all_means) - np.mean(all_means)) ** 2)
    r2 = 1 - ss_res / ss_tot if ss_tot > 0 else 1.0
    ax.plot(x_ref, np.polyval(coeffs, x_ref), color='#880E4F',
            linewidth=1.6, linestyle='-.', alpha=0.7,
            label=f'线性拟合  R²={r2:.4f}')

    # 扩展倍率标注
    for n, mean_v in zip([1, 2, 3], all_means):
        ratio = mean_v / baseline
        ax.annotate(f'{mean_v:.0f}\n({ratio:.2f}×)',
                    xy=(n, mean_v), xytext=(n + 0.12, mean_v + baseline * 0.05),
                    fontsize=9, color='#333', fontweight='bold')

    ax.set_xticks([1, 2, 3])
    ax.set_xticklabels(['N=1\n(child4)', 'N=2\n(child4+child1)', 'N=3\n(全部三链)'], fontsize=10)
    ax.set_xlabel('并发子链数量 (N)', fontsize=12)
    ax.set_ylabel('聚合吞吐量（提交次数 / Epoch）', fontsize=12)
    ax.set_xlim(0.5, 3.8)
    ax.set_ylim(0, max(all_means) * 1.35)
    ax.set_title(f'实验D：多链并发吞吐量线性扩展验证\n'
                 f'各链 50 workers（场景d配置）× {n_ep} 个稳态 Epoch，R²={r2:.4f}',
                 fontsize=11, fontweight='bold')
    ax.legend(loc='upper left', fontsize=9, framealpha=0.92)
    ax.grid(alpha=0.3)

    plt.tight_layout()
    out = os.path.join(OUT_DIR, 'fig6_linear_scaling.png')
    fig.savefig(out)
    plt.close()
    print(f'[saved] {out}')


# ─────────────────────────────────────────────────────────────────────────────
# 图7a：实验E — 资金锁定比例时序对比
# ─────────────────────────────────────────────────────────────────────────────
def fig7a_liquidity_ratio(csv_path=None):
    if csv_path is None:
        csv_path = os.path.join(os.path.dirname(__file__), '..', 'docs', 'figures', 'data',
                                'exp_e_fund_state_v5.csv')

    rows = []
    with open(csv_path) as f:
        rows = list(csv.DictReader(f))
    if not rows:
        print('[warn] fig7a: 无数据')
        return

    # ── 常量 ──────────────────────────────────────────────────────────────────
    # 6 条子链预算合计：1500+2+40000+5000+0.5+25000 = 71502.5 UNIT/epoch
    SUM_BUDGET   = 71502.5
    T_PLANNED    = 3
    TRAD_INITIAL = T_PLANNED * SUM_BUDGET   # 214507.5 UNIT

    # BillSettled 精确事件（从 metrics_fund.log 提取，含 task_id）
    # 格式：(elapsed_min, chain_name)
    BILL_EVENTS = [
        (2.13,  'child1'), (3.74,  'child6'), (6.54,  'child5'),
        (6.64,  'child3'), (6.84,  'child4'), (14.14, 'child1'),
        (15.74, 'child6'), (18.54, 'child5'), (18.64, 'child3'),
        (18.84, 'child4'), (26.14, 'child1'), (27.94, 'child6'),
        (30.64, 'child3'), (30.84, 'child4'),
    ]  # 仅截取 T=3 计划窗口内（≤31 min）的事件

    # ── 解析时序数据 ──────────────────────────────────────────────────────────
    t0 = datetime.fromisoformat(rows[0]['timestamp'].replace('Z', '+00:00'))

    elapsed  = []
    fb_ratio = []   # task_locked / TRAD_INITIAL（与传统方案同分母）

    for r in rows:
        ts = datetime.fromisoformat(r['timestamp'].replace('Z', '+00:00'))
        t  = (ts - t0).total_seconds() / 60.0
        locked = float(r['task_locked_unit'])
        elapsed.append(t)
        fb_ratio.append(locked / TRAD_INITIAL * 100)

    # 传统基线（分段线性：每次 BillSettled 降一级）
    # 等价于 (T - relative_epoch) × SUM_BUDGET / TRAD_INITIAL × 100
    trad_ratio = [float(r['baseline_locked_unit']) / TRAD_INITIAL * 100 for r in rows]

    # 找 baseline 降到 0 的时刻（T_PLANNED 完成点）
    t_end = next((elapsed[i] for i, r in enumerate(rows)
                  if float(r['baseline_locked_unit']) == 0), elapsed[-1])
    x_max = t_end + 2.5

    # 只保留截断窗口内的数据
    cut = next((i for i, t in enumerate(elapsed) if t > x_max), len(elapsed))
    elapsed_cut  = elapsed[:cut]
    fb_ratio_cut = fb_ratio[:cut]
    trad_cut     = trad_ratio[:cut]

    # ── 绘图 ──────────────────────────────────────────────────────────────────
    fig, ax = plt.subplots(figsize=(11, 5))

    ax.plot(elapsed_cut, fb_ratio_cut, color='#1565C0', linewidth=2.2,
            label='FishboneChain', zorder=3)
    ax.step(elapsed_cut, trad_cut, where='post', color='#E65100', linewidth=2.2,
            linestyle='--', label='传统预锁方案', zorder=3)

    # 节省空间填充
    ax.fill_between(elapsed_cut, fb_ratio_cut, trad_cut,
                    where=[tr > fb for tr, fb in zip(trad_cut, fb_ratio_cut)],
                    alpha=0.07, color='#E65100')

    # BillSettled 垂直标线（6 链各色）
    CHAIN_COLORS = {
        'child1': '#1E88E5', 'child2': '#43A047', 'child3': '#8E24AA',
        'child4': '#E65100', 'child5': '#00838F', 'child6': '#2E7D32',
    }
    CHAIN_LABELS = {
        'child1': 'C1(1.5k)', 'child2': 'C2(2)',  'child3': 'C3(40k)',
        'child4': 'C4(5k)',   'child5': 'C5(0.5)', 'child6': 'C6(25k)',
    }
    labeled = set()
    for (t_ev, chain) in BILL_EVENTS:
        if t_ev > x_max:
            continue
        clr = CHAIN_COLORS[chain]
        ax.axvline(x=t_ev, color=clr, linewidth=1.1, alpha=0.6, linestyle=':')
        labeled.add(chain)

    # T=3 完成线
    ax.axvline(x=t_end, color='gray', linewidth=1.3, linestyle='-.', alpha=0.7, zorder=2)
    ax.text(t_end + 0.2, 95, 'T=3 完成', fontsize=8, color='gray', va='top')

    # 改善比标注（初始时刻）
    ax.annotate('', xy=(0.5, fb_ratio_cut[0] + 1), xytext=(0.5, trad_cut[0] - 1),
                arrowprops=dict(arrowstyle='<->', color='#555', lw=1.2))
    ax.text(1.0, (fb_ratio_cut[0] + trad_cut[0]) / 2,
            f'3× 改善', fontsize=9, color='#555', va='center', fontweight='bold')

    ax.set_xlabel('运行时长（分钟）', fontsize=12)
    ax.set_ylabel('锁定资金占传统初始锁仓比例 (%)', fontsize=12)
    ax.set_title(
        '实验E：6 条并发子链  —  FishboneChain vs 传统预锁方案  锁定资金比例时序对比\n'
        'C1:1,500U  C2:2U  C3:40,000U  C4:5,000U  C5:0.5U  C6:25,000U  '
        '（Σ=71,502.5 U/epoch，T_planned=3）',
        fontsize=10, fontweight='bold')
    ax.set_xlim(0, x_max)
    ax.set_ylim(0, 108)
    ax.legend(loc='upper right', fontsize=10, framealpha=0.92,
              ncol=1, borderpad=0.8)
    ax.grid(axis='y', alpha=0.3)
    ax.grid(axis='x', alpha=0.15)

    plt.tight_layout()
    out = os.path.join(OUT_DIR, 'fig7a_liquidity_ratio.png')
    fig.savefig(out)
    plt.close()
    print(f'[saved] {out}')


# ─────────────────────────────────────────────────────────────────────────────
# 图7b：实验E — 相同总资金下的资金容量对比
# ─────────────────────────────────────────────────────────────────────────────
def fig7b_capital_capacity():
    # 参考基准：传统方案一次性预锁 T×ΣB = 3×71502.5 = 214507.5 UNIT
    # FishboneChain：仅锁 1 epoch × ΣB = 71502.5 UNIT，其余 143005 UNIT 保持可用
    SUM_BUDGET    = 71502.5
    T_PLANNED     = 3
    TOTAL_DEPOSIT = T_PLANNED * SUM_BUDGET   # 214507.5（传统方案恰好需要的总量）

    FB_LOCKED   = SUM_BUDGET                      # 71502.5
    FB_FREE     = TOTAL_DEPOSIT - FB_LOCKED        # 143005.0
    TRAD_LOCKED = TOTAL_DEPOSIT                    # 214507.5（全部锁住）
    TRAD_FREE   = 0.0

    CHILD3_BUDGET = 40000   # 用最大规格（child3 医疗标注）作为额外任务单位

    fig, ax = plt.subplots(figsize=(9, 3.5))

    methods     = ['FishboneChain', '传统预锁方案']
    locked_vals = [FB_LOCKED,  TRAD_LOCKED]
    free_vals   = [FB_FREE,    TRAD_FREE]

    ax.barh(methods, locked_vals, color=['#1565C0', '#E65100'],
            height=0.45, label='已锁定（当前 Epoch）')
    ax.barh(methods, free_vals, left=locked_vals,
            color=['#90CAF9', '#FFCC80'],
            height=0.45, label='可用余额')

    # 数值标注（传统方案：右侧；FishboneChain：各段中央）
    # 传统方案仅在右边缘标锁定量
    ax.text(TRAD_LOCKED - TOTAL_DEPOSIT * 0.01, 1,
            f'{TRAD_LOCKED:,.0f} U (100%)',
            ha='right', va='center', color='white', fontsize=9, fontweight='bold')
    # FishboneChain 两段各自标注
    ax.text(FB_LOCKED / 2, 0, f'{FB_LOCKED:,.0f} U\n({FB_LOCKED/TOTAL_DEPOSIT*100:.1f}%)',
            ha='center', va='center', color='white', fontsize=9, fontweight='bold')
    ax.text(FB_LOCKED + FB_FREE / 2, 0,
            f'{FB_FREE:,.0f} U\n({FB_FREE/TOTAL_DEPOSIT*100:.1f}%)',
            ha='center', va='center', color='#333', fontsize=9, fontweight='bold')

    # 可扩展任务标注
    new_tasks_fb = int(FB_FREE / CHILD3_BUDGET)
    ax.annotate(f'可额外激活 {new_tasks_fb} 个 child3 规格任务\n（各 40,000 U/epoch）',
                xy=(TOTAL_DEPOSIT, 0), xytext=(TOTAL_DEPOSIT * 0.65, -0.40),
                arrowprops=dict(arrowstyle='->', color='#1565C0', lw=1.5),
                fontsize=9, color='#1565C0', fontweight='bold')
    # 传统方案：直接在条形内注明
    ax.text(TOTAL_DEPOSIT * 0.5, 1,
            '无法激活任何新任务（资金全部锁定）',
            ha='center', va='center', color='white', fontsize=9,
            fontstyle='italic', alpha=0.9)

    ax.set_xlabel('资金量（UNIT）', fontsize=11)
    ax.set_title(
        f'实验E：相同总预算（{TOTAL_DEPOSIT:,.0f} UNIT = 3 epoch × Σ预算）下的资金利用率对比\n'
        '（6 条子链并发：Σ=71,502.5 U/epoch）',
        fontsize=10, fontweight='bold')
    ax.set_xlim(0, TOTAL_DEPOSIT * 1.38)
    ax.legend(loc='lower right', fontsize=10)
    ax.grid(axis='x', alpha=0.3)
    ax.set_axisbelow(True)

    plt.tight_layout()
    out = os.path.join(OUT_DIR, 'fig7b_capital_capacity.png')
    fig.savefig(out)
    plt.close()
    print(f'[saved] {out}')


if __name__ == '__main__':
    import sys
    only_fig6 = '--fig6' in sys.argv
    only_fig7 = '--fig7' in sys.argv

    if only_fig7:
        print('生成图7（资金流动性）...')
        fig7a_liquidity_ratio()
        fig7b_capital_capacity()
    elif only_fig6:
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
        try:
            fig7a_liquidity_ratio()
            fig7b_capital_capacity()
        except Exception as e:
            print(f'[warn] fig7 错误: {e}')
    print('完成。图表已保存至 docs/figures/')
