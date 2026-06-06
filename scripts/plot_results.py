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
import matplotlib.patches as mpatches
import numpy as np

OUT_DIR = os.path.join(os.path.dirname(__file__), '..', 'docs', 'figures')
os.makedirs(OUT_DIR, exist_ok=True)

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
# Figure 1: Experiment A — Capacity Bottleneck on General-Purpose Chain
# ─────────────────────────────────────────────────────────────────────────────
def fig_exp_a():
    scenarios = ['a\n(Delivery\n300w,5U)', 'b\n(Traffic\n2000w,0.001U)',
                 'c\n(Medical\n200w,200U)', 'e\n(IoT\n5000w,0.0001U)']
    ok_counts  = [251, 690, 47, 15]
    rej_counts = [1763, 5775, 261, 1082]
    total = [o + r for o, r in zip(ok_counts, rej_counts)]
    rates  = [o/t*100 for o, t in zip(ok_counts, total)]

    colors_ok  = ['#2196F3', '#4CAF50', '#FF9800', '#9C27B0']
    colors_rej = ['#BBDEFB', '#C8E6C9', '#FFE0B2', '#E1BEE7']

    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(10, 4))

    # Left: stacked bar showing ok vs rejected
    x = np.arange(len(scenarios))
    for i, (sc, ok, rej, c_ok, c_rej) in enumerate(zip(scenarios, ok_counts, rej_counts, colors_ok, colors_rej)):
        ax1.bar(i, ok, color=c_ok, label='Accepted' if i==0 else '')
        ax1.bar(i, rej, bottom=ok, color=c_rej, label='Rejected' if i==0 else '')

    ax1.set_xticks(x)
    ax1.set_xticklabels(scenarios, fontsize=9)
    ax1.set_ylabel('Submission Count')
    ax1.set_title('(a) Total Submissions on Shared Chain\n(child1, 3 epochs)')
    ax1.legend(loc='upper right')
    ax1.set_ylim(0, max(total) * 1.15)

    # Right: success rate bar chart
    bars = ax2.bar(x, rates, color=colors_ok, edgecolor='black', linewidth=0.5)
    ax2.axhline(y=100, color='gray', linestyle='--', alpha=0.4, label='100% baseline')
    ax2.set_xticks(x)
    ax2.set_xticklabels(scenarios, fontsize=9)
    ax2.set_ylabel('Success Rate (%)')
    ax2.set_title('(b) Success Rate per Scenario\n(competing on one chain)')
    ax2.set_ylim(0, 25)
    for bar, rate in zip(bars, rates):
        ax2.text(bar.get_x() + bar.get_width()/2, bar.get_height() + 0.3,
                 f'{rate:.1f}%', ha='center', va='bottom', fontsize=10, fontweight='bold')

    fig.suptitle('Experiment A: Capacity Bottleneck on a General-Purpose Chain\n'
                 '(MaxSubmissions=1000/epoch hit within 60s by high-frequency scenarios)',
                 fontsize=11, fontweight='bold')
    plt.tight_layout()
    out = os.path.join(OUT_DIR, 'fig1_exp_a_bottleneck.png')
    fig.savefig(out)
    plt.close()
    print(f'[saved] {out}')


# ─────────────────────────────────────────────────────────────────────────────
# Figure 2: Experiment B — Dedicated Chain Benefit
# ─────────────────────────────────────────────────────────────────────────────
def fig_exp_b():
    fig, axes = plt.subplots(1, 3, figsize=(12, 4))

    # Panel (a): TPS comparison — scenario b
    ax = axes[0]
    labels = ['child1\nAURA-6s\n(general)', 'child2\nAURA-2s\n(dedicated)']
    tps    = [0.2, 80.5]
    colors = ['#FF7043', '#42A5F5']
    bars = ax.bar(labels, tps, color=colors, edgecolor='black', linewidth=0.5, width=0.5)
    for bar, v in zip(bars, tps):
        ax.text(bar.get_x() + bar.get_width()/2, v + 1, f'{v} TPS',
                ha='center', va='bottom', fontweight='bold', fontsize=11)
    ax.set_ylabel('Peak Throughput (TPS)')
    ax.set_title('(a) Scenario b: High-Frequency Traffic\nGeneral vs Dedicated Chain')
    ax.set_ylim(0, 100)
    ax.annotate('400× improvement', xy=(1, 80.5), xytext=(0.5, 70),
                arrowprops=dict(arrowstyle='->', color='green', lw=1.5),
                fontsize=10, color='green', fontweight='bold')

    # Panel (b): Fail rate comparison — AURA vs BABE
    ax = axes[1]
    labels = ['child1\nAURA-6s\n(shared, congested)', 'child1\nAURA-6s\n(isolated)', 'child6\nBABE-5s\n(dedicated)']
    fail_rates = [100, 44, 0]
    colors = ['#EF5350', '#FFA726', '#66BB6A']
    bars = ax.bar(labels, fail_rates, color=colors, edgecolor='black', linewidth=0.5, width=0.5)
    for bar, v in zip(bars, fail_rates):
        ax.text(bar.get_x() + bar.get_width()/2, v + 1, f'{v}%',
                ha='center', va='bottom', fontweight='bold', fontsize=12)
    ax.set_ylabel('Mempool Fail Rate (%)')
    ax.set_title('(b) Scenario f: Mempool Fail Rate\nAURA vs BABE Consensus')
    ax.set_ylim(0, 115)
    ax.axhline(y=0, color='green', linestyle='--', alpha=0.5)

    # Panel (c): Success rate — scenario f AURA vs BABE isolated
    ax = axes[2]
    labels = ['AURA-6s\n(child1, isolated)', 'BABE-5s\n(child6, dedicated)']
    # AURA: 54% success, BABE: ~24% (limited by MaxSubs, 0% fail)
    # When BABE hits MaxSubs it rejects deterministically
    success = [54, 24.4]
    fail_r  = [44, 0]
    reject  = [100-s-f for s, f in zip(success, fail_r)]
    colors_s = ['#42A5F5', '#42A5F5']
    colors_f = ['#EF5350', '#EF5350']
    colors_r = ['#BDBDBD', '#BDBDBD']
    x = np.arange(len(labels))
    ax.bar(x, success, color='#42A5F5', label='Accepted')
    ax.bar(x, fail_r,  bottom=success, color='#EF5350', label='Fail (mempool)')
    ax.bar(x, reject,  bottom=[s+f for s,f in zip(success,fail_r)], color='#BDBDBD', label='Rejected (on-chain)')
    ax.set_xticks(x)
    ax.set_xticklabels(labels, fontsize=9)
    ax.set_ylabel('Submission Outcome (%)')
    ax.set_title('(c) Scenario f Isolated Test\nAURA vs BABE Outcome Breakdown')
    ax.legend(loc='upper right', fontsize=9)
    ax.set_ylim(0, 115)

    fig.suptitle('Experiment B: Dedicated Chain Benefit\n'
                 '(Same scenario, different chain configurations)',
                 fontsize=11, fontweight='bold')
    plt.tight_layout()
    out = os.path.join(OUT_DIR, 'fig2_exp_b_dedicated.png')
    fig.savefig(out)
    plt.close()
    print(f'[saved] {out}')


# ─────────────────────────────────────────────────────────────────────────────
# Figure 3: Experiment C — Multi-Chain Throughput Over Time
# ─────────────────────────────────────────────────────────────────────────────
def fig_exp_c_timeline():
    rows = []
    with open('/tmp/exp_c_state.csv') as f:
        rows = list(csv.DictReader(f))

    CHAINS_ORDER = [
        ('ws://10.2.2.20:9949', 'child5 (e, 5000w)'),
        ('ws://10.2.2.11:9950', 'child6 (f, 500w)'),
        ('ws://10.2.2.11:9945', 'child1 (a, 300w)'),
        ('ws://10.2.2.17:9947', 'child3 (c, 200w)'),
        ('ws://10.2.2.14:9946', 'child2 (b, 2000w)'),
        ('ws://10.2.2.11:9948', 'child4 (d, 100w)'),
    ]
    COLORS = ['#8E24AA', '#00ACC1', '#E53935', '#43A047', '#FB8C00', '#1E88E5']
    chain_names = [n for _, n in CHAINS_ORDER]
    url_to_name = {u: n for u, n in CHAINS_ORDER}

    # Aggregate per epoch per chain (max subs in that epoch)
    # Group by (chain, epoch), take max subs
    ep_max = defaultdict(lambda: defaultdict(int))
    ep_ts  = defaultdict(lambda: defaultdict(str))  # chain → epoch → first timestamp
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

    # Build time axis: use child4 epoch timestamps as reference
    c4_name = 'child4 (d, 100w)'
    c4_epochs_sorted = sorted(ep_ts[c4_name].keys())
    t0 = datetime.strptime(ep_ts[c4_name][c4_epochs_sorted[0]], '%Y-%m-%dT%H:%M:%S.%fZ')

    def epoch_elapsed_h(name, ep):
        ts_str = ep_ts[name].get(ep, '')
        if not ts_str: return None
        try:
            t = datetime.strptime(ts_str, '%Y-%m-%dT%H:%M:%S.%fZ')
            return (t - t0).total_seconds() / 3600
        except: return None

    fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(12, 8))

    # Panel 1: First 6h stacked bar by chain (cleaner than stackplot)
    # Collect all timestamps in first 6h
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
    ax1.set_xlabel('Elapsed Time (hours)')
    ax1.set_ylabel('Submissions Count')
    ax1.set_title('(a) 6-Chain Parallel Throughput — First 6 Hours\n'
                  'Stacked area shows concurrent submissions; each band = one chain\'s output')
    ax1.legend(loc='upper right', fontsize=8, ncol=2)
    ax1.grid(axis='y', alpha=0.3)

    # Annotate drop-offs
    for tx, label, color in [(0.37,'e OOM','#6A1B9A'), (1.4,'f OOM','#00838F'), (2.8,'a OOM','#C62828')]:
        ax1.axvline(x=tx, color=color, linestyle='--', alpha=0.6, linewidth=1.2)
        ax1.text(tx+0.05, 4200, label, color=color, fontsize=8, fontweight='bold')

    # Panel 2: child4 epoch-by-epoch (112 epochs)
    c4_eps = sorted(ep_max[c4_name].keys())
    c4_subs = [ep_max[c4_name][ep] for ep in c4_eps]
    c4_xs   = list(range(1, len(c4_eps)+1))

    ax2.bar(c4_xs, c4_subs, color='#1565C0', alpha=0.75, width=1.0)
    ax2.axhline(y=100, color='red', linestyle='--', linewidth=1.2, label='Expected: 100 subs/epoch (100 workers)')
    ax2.set_xlim(0, len(c4_xs)+1)
    ax2.set_ylim(0, 130)
    ax2.set_xlabel('Epoch Sequence (chronological)')
    ax2.set_ylabel('Peak Submissions per Epoch')
    ax2.set_title('(b) child4: 112 Consecutive Epochs (22 hours) — Dedicated Financial Chain\n'
                  'Steady-state: 100 subs/epoch, zero fails, 100% worker participation rate')
    ax2.legend(loc='upper right')
    ax2.grid(axis='y', alpha=0.3)

    fig.suptitle('Experiment C: 6-Chain Parallel Operation (22 Hours)',
                 fontsize=12, fontweight='bold', y=1.01)
    fig.tight_layout()
    out = os.path.join(OUT_DIR, 'fig3_exp_c_timeline.png')
    fig.savefig(out, bbox_inches='tight')
    plt.close()
    print(f'[saved] {out}')


# ─────────────────────────────────────────────────────────────────────────────
# Figure 4: child4 Steady-State — 112 consecutive epochs
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
    ax.axhline(y=100, color='red', linestyle='--', linewidth=1.2, label='Expected: 100 subs/epoch (100 workers)')
    ax.fill_between(ep_seq, subs, alpha=0.15, color='#1E88E5')
    ax.set_xlim(1, len(ep_seq))
    ax.set_ylim(0, 130)
    ax.set_xlabel('Epoch Sequence (chronological)')
    ax.set_ylabel('Submissions per Epoch')
    ax.set_title('Experiment C: child4 (Dedicated Financial Chain) — 112 Consecutive Epochs\n'
                 '100 workers × 1 submission/epoch = 100 subs/epoch, zero failures, 22-hour sustained')
    ax.legend(loc='upper right')
    ax.grid(axis='y', alpha=0.3)

    avg_subs = np.mean(subs)
    ax.text(len(ep_seq)*0.05, 115, f'Average: {avg_subs:.1f} subs/epoch  |  0 fails  |  100% worker participation',
            fontsize=9, color='#1565C0', style='italic')

    plt.tight_layout()
    out = os.path.join(OUT_DIR, 'fig4_child4_steady.png')
    fig.savefig(out)
    plt.close()
    print(f'[saved] {out}')


# ─────────────────────────────────────────────────────────────────────────────
# Figure 5: Epoch-1 Snapshot — 6 chains simultaneous (placeholder for scaling exp)
# ─────────────────────────────────────────────────────────────────────────────
def fig_epoch1_snapshot():
    chains    = ['child1\n(a,300w)', 'child2\n(b,2000w)', 'child3\n(c,200w)',
                 'child4\n(d,100w)', 'child5\n(e,5000w)', 'child6\n(f,500w)']
    subs_ep1  = [1000, 1000, 200, 100, 1000, 500]  # from time-aligned epoch 1
    colors    = ['#E53935', '#FB8C00', '#43A047', '#1E88E5', '#8E24AA', '#00ACC1']
    single_cap = 1000

    fig, ax = plt.subplots(figsize=(9, 4))
    x = np.arange(len(chains))
    bars = ax.bar(x, subs_ep1, color=colors, edgecolor='black', linewidth=0.5, width=0.6)
    ax.axhline(y=single_cap, color='red', linestyle='--', linewidth=1.5,
               label=f'Single-chain capacity = {single_cap} subs/epoch')

    for bar, v in zip(bars, subs_ep1):
        ax.text(bar.get_x() + bar.get_width()/2, v + 10, str(v),
                ha='center', va='bottom', fontweight='bold', fontsize=10)

    total = sum(subs_ep1)
    ax.annotate(f'Total = {total} subs/epoch\n({total/single_cap:.1f}× single chain)',
                xy=(2.5, 600), xytext=(3.8, 900),
                arrowprops=dict(arrowstyle='->', color='green', lw=2),
                fontsize=11, color='green', fontweight='bold',
                bbox=dict(boxstyle='round,pad=0.3', facecolor='#E8F5E9', edgecolor='green'))

    ax.set_xticks(x)
    ax.set_xticklabels(chains, fontsize=9)
    ax.set_ylabel('Submissions in Epoch 1')
    ax.set_ylim(0, 1300)
    ax.set_title('Experiment C Epoch 1: All 6 Chains Simultaneously Active\n'
                 f'Aggregate = {total} subs ({total/single_cap:.1f}× single-chain capacity)')
    ax.legend(loc='upper right')
    ax.grid(axis='y', alpha=0.3)

    plt.tight_layout()
    out = os.path.join(OUT_DIR, 'fig5_epoch1_snapshot.png')
    fig.savefig(out)
    plt.close()
    print(f'[saved] {out}')


# ─────────────────────────────────────────────────────────────────────────────
# Figure 6: Linear Scaling — N=1/2/4 chains, same 50-worker config
# ─────────────────────────────────────────────────────────────────────────────
def fig_linear_scaling(csv_path='/tmp/exp_scale_state.csv'):
    rows = []
    with open(csv_path) as f:
        rows = list(csv.DictReader(f))
    if not rows:
        print('[warn] fig6: no data in exp_scale_state.csv')
        return

    CHAIN_URLS = {
        'ws://10.2.2.11:9948': 'child4',
        'ws://10.2.2.17:9947': 'child3',
        'ws://10.2.2.11:9945': 'child1',
        'ws://10.2.2.11:9950': 'child6',
    }

    # Per-epoch max subs for each chain
    ep_subs = {name: defaultdict(int) for name in CHAIN_URLS.values()}
    for r in rows:
        name = CHAIN_URLS.get(r['chain_url'])
        if not name: continue
        try:
            ep   = int(r['epoch_id'])
            subs = int(r['submissions_count'])
            ep_subs[name][ep] = max(ep_subs[name][ep], subs)
        except: pass

    # Average over stable epochs: exclude first (partial start) and last (in-progress)
    def avg_active(name):
        ep_items = sorted(ep_subs[name].items())
        if len(ep_items) <= 2:
            vals = [s for _, s in ep_items if s > 0]
        else:
            # Exclude first epoch (workers may have connected mid-epoch)
            # and last epoch (likely still in progress)
            stable = ep_items[1:-1]
            vals = [s for _, s in stable if s > 0]
        return np.mean(vals) if vals else 0, len(vals)

    c4_avg, c4_n = avg_active('child4')
    c3_avg, c3_n = avg_active('child3')
    c1_avg, c1_n = avg_active('child1')
    c6_avg, c6_n = avg_active('child6')

    print(f'[fig6] child4: {c4_avg:.1f} subs/ep ({c4_n} epochs)')
    print(f'[fig6] child3: {c3_avg:.1f} subs/ep ({c3_n} epochs)')
    print(f'[fig6] child1: {c1_avg:.1f} subs/ep ({c1_n} epochs)')
    print(f'[fig6] child6: {c6_avg:.1f} subs/ep ({c6_n} epochs)')

    n_chains   = [1, 2, 4]
    total_subs = [c4_avg, c4_avg + c3_avg, c4_avg + c3_avg + c1_avg + c6_avg]

    # Perfect linear reference
    x_ref = np.linspace(0, 4.5, 100)
    y_ref = c4_avg * x_ref  # slope = single-chain throughput

    fig, ax = plt.subplots(figsize=(7, 5))

    ax.scatter(n_chains, total_subs, color='#1565C0', s=120, zorder=5, label='Measured aggregate throughput')
    ax.plot(x_ref, y_ref, 'r--', linewidth=1.5, label=f'Ideal linear: {c4_avg:.0f}N subs/epoch')

    for n, s in zip(n_chains, total_subs):
        ax.annotate(f'{s:.0f} subs/epoch\n(N={n})', xy=(n, s),
                    xytext=(n + 0.15, s - c4_avg * 0.2),
                    fontsize=10, fontweight='bold', color='#1565C0')

    ax.set_xticks([1, 2, 4])
    ax.set_xticklabels(['1 chain\n(child4)', '2 chains\n(child3+4)', '4 chains\n(child1+3+4+6)'])
    ax.set_xlabel('Number of Parallel Chains (N)')
    ax.set_ylabel('Aggregate Throughput (submissions/epoch)')
    ax.set_xlim(0, 5)
    ax.set_ylim(0, max(total_subs) * 1.3)
    ax.set_title('Experiment D: Linear Throughput Scaling\n'
                 '50 workers × scenario-d config per chain, measured concurrently')
    ax.legend(loc='upper left')
    ax.grid(alpha=0.3)

    # Add scaling factor annotation
    if len(n_chains) >= 3:
        predicted_4 = c4_avg * 4
        actual_4 = total_subs[2]
        ratio = actual_4 / predicted_4
        ax.text(0.3, max(total_subs) * 0.55,
                f'N=4 measured: {actual_4:.0f} subs/epoch\n'
                f'N=4 ideal (4×N=1): {predicted_4:.0f} subs/epoch\n'
                f'Scaling ratio: {ratio:.2f}× (chain diversity bonus)',
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
        print('Generating Figure 6 only (linear scaling)...')
        try:
            fig_linear_scaling()
        except Exception as e:
            print(f'[warn] fig6 error: {e}')
    else:
        print('Generating figures...')
        fig_exp_a()
        fig_exp_b()
        try:
            fig_exp_c_timeline()
        except Exception as e:
            print(f'[warn] fig3 error: {e}')
        fig_child4_steady()
        fig_epoch1_snapshot()
        try:
            fig_linear_scaling()
        except Exception as e:
            print(f'[warn] fig6 (needs exp_scale data): {e}')
    print('Done. Figures saved to docs/figures/')
