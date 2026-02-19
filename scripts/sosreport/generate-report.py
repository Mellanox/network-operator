#!/usr/bin/env python3

#  2026 NVIDIA CORPORATION & AFFILIATES
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

"""NVIDIA Network Operator SOS-Report HTML Report Generator (Python).

Reads a sosreport collection directory and generates a self-contained HTML
report for interactive browsing of the diagnostic data.

Usage:
    ./generate-report.py <sosreport-dir> [--output <path>] [--template <path>]

Default output: <sosreport-dir>/report.html
"""

import argparse
import glob
import html
import os
import re
import string
import sys
from pathlib import Path


# ============================================================================
# Helper Functions
# ============================================================================

def read_file(path):
    """Read a file and return its content as a string. Return empty string if missing."""
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as f:
            return f.read()
    except (OSError, IOError):
        return ""


def extract_yaml_field(content, field):
    """Extract a YAML field value using regex.

    Mimics: grep -m1 "^[[:space:]]*field:" | sed extraction
    Returns the value after "field:" (first match, trimmed, quotes stripped).
    """
    match = re.search(
        r"^\s*" + re.escape(field) + r":\s*(.*?)\s*$", content, re.MULTILINE
    )
    if match:
        val = match.group(1)
        # Strip surrounding quotes
        if val.startswith('"') and val.endswith('"'):
            val = val[1:-1]
        return val
    return ""


def status_badge(state):
    """Return an HTML badge span for the given state."""
    ready_states = {"ready", "Ready", "Running", "Active", "True", "Healthy"}
    warn_states = {"notReady", "NotReady", "Pending", "Progressing"}
    error_states = {
        "error", "Error", "Failed", "CrashLoopBackOff",
        "ImagePullBackOff", "ErrImagePull",
    }

    if state in ready_states:
        css_class = "badge-ready"
    elif state in warn_states:
        css_class = "badge-warn"
    elif state in error_states:
        css_class = "badge-error"
    else:
        css_class = "badge-ignore"

    return f'<span class="badge {css_class}">{html.escape(state)}</span>'


def collapsible(title, content, css_class=""):
    """Wrap content in a <details>/<summary> collapsible block.

    Content is placed inside <pre><code>...</code></pre>.
    """
    line_count = content.count("\n") + (1 if content and not content.endswith("\n") else 0)
    if content.endswith("\n"):
        line_count = content.count("\n")
    # Match bash: echo "$content" | wc -l
    # wc -l counts newline characters, so a trailing newline means the last line is counted
    line_count = len(content.split("\n"))

    extra = f" {css_class}" if css_class else ""
    return (
        f'<details class="collapsible{extra}">\n'
        f'<summary>{title} <span class="line-count">{line_count} lines</span></summary>\n'
        f'<div class="collapsible-content">\n'
        f"<pre><code>{content}</code></pre>\n"
        f"</div>\n"
        f"</details>\n"
    )


def file_to_pre(path, css_class="yaml-block"):
    """Read a file, html.escape() the content, wrap in <pre><code>."""
    content = read_file(path)
    if content:
        escaped = html.escape(content)
    else:
        escaped = "(not collected)"
    return f'<pre class="{css_class}"><code>{escaped}</code></pre>'


def file_content_escaped(path):
    """Read file and return HTML-escaped content, or placeholder if missing/empty."""
    content = read_file(path)
    if content.strip():
        return html.escape(content)
    return "(not collected)"


def highlight_logs(text):
    """Add log-error/log-warn spans around error/warning/panic/fatal lines.

    Input should already be HTML-escaped.
    """
    lines = text.split("\n")
    result = []
    for line in lines:
        if re.search(r"[Ee][Rr][Rr][Oo][Rr]", line):
            result.append(f'<span class="log-error">{line}</span>')
        elif re.search(r"[Pp][Aa][Nn][Ii][Cc]", line):
            result.append(f'<span class="log-error">{line}</span>')
        elif re.search(r"[Ff][Aa][Tt][Aa][Ll]", line):
            result.append(f'<span class="log-error">{line}</span>')
        elif re.search(r"[Ww][Aa][Rr][Nn]", line):
            result.append(f'<span class="log-warn">{line}</span>')
        else:
            result.append(line)
    return "\n".join(result)


def count_files(pattern):
    """Count files matching a glob pattern."""
    return len(glob.glob(pattern))


def sorted_dirs(parent):
    """Return sorted list of subdirectory paths under parent."""
    if not os.path.isdir(parent):
        return []
    entries = []
    for name in sorted(os.listdir(parent)):
        full = os.path.join(parent, name)
        if os.path.isdir(full):
            entries.append(full)
    return entries


def sorted_files(parent, extension=""):
    """Return sorted list of file paths under parent, optionally filtered by extension."""
    if not os.path.isdir(parent):
        return []
    entries = []
    for name in sorted(os.listdir(parent)):
        full = os.path.join(parent, name)
        if os.path.isfile(full):
            if extension and not name.endswith(extension):
                continue
            entries.append(full)
    return entries


# ============================================================================
# Section Renderers
# ============================================================================

def render_dashboard(report_dir):
    """Render page header and dashboard cards."""
    d = report_dir

    # Extract metadata
    collection_time = ""
    script_version = ""
    operator_ns = ""
    platform = ""
    cluster_context = ""

    info_file = os.path.join(d, "metadata", "collection-info.txt")
    if os.path.isfile(info_file):
        info_content = read_file(info_file)
        m = re.search(r"Collection Time:\s*(.*)", info_content, re.IGNORECASE)
        if m:
            collection_time = m.group(1).strip()
        m = re.search(r"Script Version:\s*(.*)", info_content, re.IGNORECASE)
        if m:
            script_version = m.group(1).strip()
        m = re.search(r"Operator Namespace:\s*(.*)", info_content, re.IGNORECASE)
        if m:
            operator_ns = m.group(1).strip()
        m = re.search(r"Platform:\s*(.*)", info_content, re.IGNORECASE)
        if m:
            platform = m.group(1).strip()
        m = re.search(r"Context:\s*(.*)", info_content, re.IGNORECASE)
        if m:
            cluster_context = m.group(1).strip()

    # Extract NCP state
    ncp_state = "unknown"
    ncp_file = os.path.join(d, "crds", "instances", "nicclusterpolicies", "all.yaml")
    if os.path.isfile(ncp_file):
        ncp_content = read_file(ncp_file)
        # Use the last match of state: (handles List wrapper with 4-space indent)
        state_matches = re.findall(r"^\s*state:\s*(\S+)", ncp_content, re.MULTILINE)
        if state_matches:
            ncp_state = state_matches[-1]
    if not ncp_state:
        ncp_state = "unknown"

    # Extract counts from diagnostic summary
    node_count = 0
    comp_found = 0
    comp_skipped = 0
    crd_defs = 0
    crd_instances = 0
    warn_count = 0
    error_count = 0
    pod_count = 0

    summary_file = os.path.join(d, "diagnostic-summary.txt")
    if os.path.isfile(summary_file):
        summary = read_file(summary_file)

        def _extract_summary_int(pattern, text):
            m = re.search(pattern, text, re.MULTILINE)
            if m:
                try:
                    return int(m.group(1))
                except (ValueError, IndexError):
                    pass
            return 0

        node_count = _extract_summary_int(r"^Nodes:\s*(\d+)", summary)
        comp_found = _extract_summary_int(r"Components Found:\s*(\d+)", summary)
        comp_skipped = _extract_summary_int(r"Components Skipped:\s*(\d+)", summary)
        crd_defs = _extract_summary_int(r"CRD Definitions:\s*(\d+)", summary)
        crd_instances = _extract_summary_int(r"CRD Instances:\s*(\d+)", summary)
        warn_count = _extract_summary_int(r"^Warnings:\s*(\d+)", summary)
        error_count = _extract_summary_int(r"^Errors:\s*(\d+)", summary)
        pod_count = _extract_summary_int(r"Component Pods:\s*(\d+)", summary)

    # Fallback counts from filesystem
    if node_count == 0:
        nodes_summary = os.path.join(d, "nodes", "nodes-summary.txt")
        if os.path.isfile(nodes_summary):
            content = read_file(nodes_summary)
            node_count = len(re.findall(r"Ready|NotReady", content))

    if crd_defs == 0:
        crd_defs = count_files(os.path.join(d, "crds", "definitions", "*.yaml"))

    # NCP status card class
    ncp_card_class = "card"
    if ncp_state == "ready":
        ncp_card_class = "card card-ready"
    elif ncp_state == "notReady":
        ncp_card_class = "card card-warn"
    elif ncp_state == "error":
        ncp_card_class = "card card-error"

    # Error card class
    err_card_class = "card"
    if error_count > 0:
        err_card_class = "card card-error"

    # Build meta line
    meta_parts = []
    if collection_time:
        meta_parts.append(f"Collected: {html.escape(collection_time)}")
    if script_version:
        meta_parts.append(f"Version: {html.escape(script_version)}")
    if operator_ns:
        meta_parts.append(f"Namespace: {html.escape(operator_ns)}")
    if platform:
        meta_parts.append(f"Platform: {html.escape(platform)}")
    if cluster_context:
        meta_parts.append(f"Context: {html.escape(cluster_context)}")
    meta_line = " &nbsp;|&nbsp; ".join(meta_parts)

    return f"""\
<div class="page-header">
    <h1>NVIDIA Network Operator SOS-Report</h1>
    <div class="meta">
        {meta_line}
    </div>
</div>

<div class="dashboard">
    <div class="{ncp_card_class}">
        <div class="card-label">NCP Status</div>
        <div class="card-value" style="font-size:20px;">{status_badge(ncp_state)}</div>
        <div class="card-detail">NicClusterPolicy</div>
    </div>
    <div class="card">
        <div class="card-label">Nodes</div>
        <div class="card-value">{node_count}</div>
        <div class="card-detail">cluster nodes</div>
    </div>
    <div class="card">
        <div class="card-label">Components</div>
        <div class="card-value">{comp_found}</div>
        <div class="card-detail">{comp_skipped} skipped</div>
    </div>
    <div class="card">
        <div class="card-label">Pods</div>
        <div class="card-value">{pod_count}</div>
        <div class="card-detail">component pods</div>
    </div>
    <div class="card">
        <div class="card-label">CRDs</div>
        <div class="card-value">{crd_defs}</div>
        <div class="card-detail">{crd_instances} instances</div>
    </div>
    <div class="{err_card_class}">
        <div class="card-label">Issues</div>
        <div class="card-value">{error_count}</div>
        <div class="card-detail">{warn_count} warnings</div>
    </div>
</div>
"""


def render_ncp_status(report_dir):
    """Render the NicClusterPolicy status section."""
    ncp_file = os.path.join(report_dir, "crds", "instances", "nicclusterpolicies", "all.yaml")

    parts = []
    parts.append("""\
<div class="section" id="ncp-status">
<div class="section-header">
    <h2>NicClusterPolicy Status</h2>
    <div class="expand-controls">
        <button onclick="toggleAll(this,'ncp-status',true)">Expand All</button>
        <button onclick="toggleAll(this,'ncp-status',false)">Collapse All</button>
    </div>
</div>
<div class="section-body">
""")

    if not os.path.isfile(ncp_file):
        parts.append('<div class="empty-state">No NicClusterPolicy found in collection</div>\n')
    else:
        ncp_content = read_file(ncp_file)

        # Extract overall state (last match, handles List wrapper)
        state_matches = re.findall(r"^\s*state:\s*(\S+)", ncp_content, re.MULTILINE)
        ncp_state = state_matches[-1] if state_matches else "unknown"

        # Extract reason (last match)
        reason_matches = re.findall(r"^\s*reason:\s*(.*?)\s*$", ncp_content, re.MULTILINE)
        ncp_reason = reason_matches[-1] if reason_matches else ""

        parts.append(f'<p><strong>Overall State:</strong> {status_badge(ncp_state)}')
        if ncp_reason:
            parts.append(f" &mdash; {html.escape(ncp_reason)}")
        parts.append("</p>\n")

        # Extract appliedStates table
        if "appliedStates:" in ncp_content:
            parts.append('<table style="margin-top:12px;">\n')
            parts.append("<tr><th>Component</th><th>State</th><th>Message</th></tr>\n")

            # Parse appliedStates block
            # Find the appliedStates section
            applied_match = re.search(
                r"appliedStates:\s*\n((?:\s+-\s+.*\n(?:\s+\w.*\n)*)*)",
                ncp_content,
            )
            if applied_match:
                block = applied_match.group(0)
                # Parse entries: each starts with "- name:"
                entries = re.findall(
                    r"-\s+name:\s*(\S+)\s*\n(?:\s+state:\s*(\S+))?\s*\n?(?:\s+message:\s*(.*))?",
                    block,
                )
                if not entries:
                    # Alternative parsing: iterate line by line
                    in_applied = False
                    current_name = ""
                    current_state = ""
                    current_message = ""
                    for line in ncp_content.split("\n"):
                        if "appliedStates:" in line and "state:" not in line.replace("appliedStates:", ""):
                            in_applied = True
                            continue
                        if in_applied:
                            # Check if we've left the appliedStates block
                            stripped = line.lstrip()
                            if stripped and not stripped.startswith("-") and not stripped.startswith("name:") and not stripped.startswith("state:") and not stripped.startswith("message:"):
                                indent = len(line) - len(line.lstrip())
                                if indent <= 2 and stripped and not stripped.startswith("-"):
                                    # End of appliedStates block
                                    if current_name:
                                        parts.append(
                                            f"<tr><td><code>{html.escape(current_name)}</code></td>"
                                            f"<td>{status_badge(current_state)}</td>"
                                            f"<td>{html.escape(current_message)}</td></tr>\n"
                                        )
                                    in_applied = False
                                    continue

                            name_m = re.match(r"\s*-\s*name:\s*(\S+)", line)
                            if name_m:
                                # Emit previous entry
                                if current_name:
                                    parts.append(
                                        f"<tr><td><code>{html.escape(current_name)}</code></td>"
                                        f"<td>{status_badge(current_state)}</td>"
                                        f"<td>{html.escape(current_message)}</td></tr>\n"
                                    )
                                current_name = name_m.group(1)
                                current_state = ""
                                current_message = ""
                                continue

                            state_m = re.match(r"\s+state:\s*(\S+)", line)
                            if state_m and "appliedStates" not in line:
                                current_state = state_m.group(1)
                                continue

                            msg_m = re.match(r"\s+message:\s*(.*)", line)
                            if msg_m:
                                current_message = msg_m.group(1).strip()
                                continue

                    # Emit last entry
                    if in_applied and current_name:
                        parts.append(
                            f"<tr><td><code>{html.escape(current_name)}</code></td>"
                            f"<td>{status_badge(current_state)}</td>"
                            f"<td>{html.escape(current_message)}</td></tr>\n"
                        )
                else:
                    for name, state, message in entries:
                        parts.append(
                            f"<tr><td><code>{html.escape(name)}</code></td>"
                            f"<td>{status_badge(state)}</td>"
                            f"<td>{html.escape(message.strip() if message else '')}</td></tr>\n"
                        )

            parts.append("</table>\n")

        # Full YAML collapsible
        parts.append("\n")
        parts.append(collapsible(
            "NicClusterPolicy Full YAML",
            file_content_escaped(ncp_file),
            "yaml-block",
        ))

    parts.append("""\
</div>
</div>
""")
    return "".join(parts)


def render_components(report_dir):
    """Render the component health section."""
    components_dir = os.path.join(report_dir, "operator", "components")

    parts = []
    parts.append("""\
<div class="section" id="components">
<div class="section-header">
    <h2>Component Health</h2>
    <div class="expand-controls">
        <button onclick="toggleAll(this,'components',true)">Expand All</button>
        <button onclick="toggleAll(this,'components',false)">Collapse All</button>
    </div>
</div>
<div class="section-body">
""")

    if not os.path.isdir(components_dir):
        parts.append('<div class="empty-state">No component data found</div>\n')
    else:
        # Column headers
        parts.append('<div class="comp-grid-header">\n')
        parts.append(
            "<span></span><span>Component</span><span>Type</span>"
            "<span>Desired</span><span>Ready</span><span>Pods</span>"
            "<span>Restarts</span><span>Status</span>\n"
        )
        parts.append("</div>\n")

        for comp_path in sorted_dirs(components_dir):
            comp_name = os.path.basename(comp_path)

            # Determine workload type
            workload_type = "unknown"
            workload_file = ""
            ds_file = os.path.join(comp_path, "daemonset.yaml")
            dep_file = os.path.join(comp_path, "deployment.yaml")
            if os.path.isfile(ds_file):
                workload_type = "DaemonSet"
                workload_file = ds_file
            elif os.path.isfile(dep_file):
                workload_type = "Deployment"
                workload_file = dep_file

            # Extract replica counts
            desired = "-"
            ready_count = "-"
            if workload_file:
                wf_content = read_file(workload_file)
                if workload_type == "DaemonSet":
                    desired = extract_yaml_field(wf_content, "desiredNumberScheduled") or "-"
                    ready_count = extract_yaml_field(wf_content, "numberReady") or "-"
                else:
                    desired = extract_yaml_field(wf_content, "replicas") or "-"
                    ready_count = extract_yaml_field(wf_content, "readyReplicas") or "-"

            # Count pods
            pods_dir = os.path.join(comp_path, "pods")
            pod_count = 0
            if os.path.isdir(pods_dir):
                pod_count = len(glob.glob(os.path.join(pods_dir, "*.yaml")))

            # Max restart count
            max_restarts = 0
            if os.path.isdir(pods_dir):
                pod_yamls = glob.glob(os.path.join(pods_dir, "*.yaml"))
                for pf in pod_yamls:
                    pf_content = read_file(pf)
                    for rm in re.findall(r"restartCount:\s*(\d+)", pf_content):
                        try:
                            val = int(rm)
                            if val > max_restarts:
                                max_restarts = val
                        except ValueError:
                            pass

            # Determine status
            comp_status = "unknown"
            if desired != "-" and ready_count != "-":
                if desired == ready_count and ready_count != "0":
                    comp_status = "ready"
                elif ready_count == "0":
                    comp_status = "error"
                else:
                    comp_status = "notReady"
            elif pod_count > 0:
                failed_pods = 0
                if os.path.isdir(pods_dir):
                    for pf in glob.glob(os.path.join(pods_dir, "*.yaml")):
                        pf_content = read_file(pf)
                        if re.search(r"phase:\s*(Failed|CrashLoopBackOff)", pf_content):
                            failed_pods += 1
                if failed_pods > 0:
                    comp_status = "error"
                else:
                    comp_status = "ready"

            # Restart warning display
            restart_display = str(max_restarts)
            try:
                if max_restarts > 5:
                    restart_display = (
                        f'<span style="color:var(--status-warn);font-weight:600">'
                        f"{max_restarts}</span>"
                    )
            except (ValueError, TypeError):
                pass

            # Open the combined details element
            parts.append('<details class="comp-row">\n')
            parts.append('<summary class="comp-row-summary">\n')
            parts.append('<span class="expand-chevron">&#9654;</span>\n')
            parts.append(f'<span class="comp-name">{html.escape(comp_name)}</span>\n')
            parts.append(f"<span>{html.escape(workload_type)}</span>\n")
            parts.append(f"<span>{html.escape(str(desired))}</span>\n")
            parts.append(f"<span>{html.escape(str(ready_count))}</span>\n")
            parts.append(f"<span>{pod_count}</span>\n")
            parts.append(f"<span>{restart_display}</span>\n")
            parts.append(f"<span>{status_badge(comp_status)}</span>\n")
            parts.append("</summary>\n")
            parts.append('<div class="comp-row-detail">\n')

            # Workload YAML files
            for wf in [ds_file, dep_file]:
                if os.path.isfile(wf):
                    parts.append(collapsible(
                        os.path.basename(wf),
                        file_content_escaped(wf),
                        "yaml-block",
                    ))

            # Pod details
            if os.path.isdir(pods_dir):
                for pod_yaml in sorted(glob.glob(os.path.join(pods_dir, "*.yaml"))):
                    pod_name = os.path.splitext(os.path.basename(pod_yaml))[0]
                    pod_content = read_file(pod_yaml)
                    pod_phase = extract_yaml_field(pod_content, "phase") or "Unknown"
                    pod_node = extract_yaml_field(pod_content, "nodeName") or "unknown"
                    pod_restart_m = re.search(r"restartCount:\s*(\d+)", pod_content)
                    pod_restarts = pod_restart_m.group(1) if pod_restart_m else "0"

                    parts.append('<div class="pod-card">\n')
                    parts.append(
                        f'<div class="pod-name">{html.escape(pod_name)} '
                        f"{status_badge(pod_phase)}</div>\n"
                    )
                    parts.append(
                        f'<div class="pod-meta">Node: {html.escape(pod_node)} | '
                        f"Restarts: {html.escape(pod_restarts)}</div>\n"
                    )

                    # Pod YAML collapsible
                    parts.append(collapsible(
                        "Pod YAML",
                        file_content_escaped(pod_yaml),
                        "yaml-block",
                    ))

                    # Pod logs
                    log_file = os.path.join(pods_dir, f"{pod_name}.log")
                    if os.path.isfile(log_file) and os.path.getsize(log_file) > 0:
                        log_content = read_file(log_file)
                        escaped_log = highlight_logs(html.escape(log_content))
                        log_lines = len(log_content.split("\n"))
                        parts.append('<details class="collapsible">\n')
                        parts.append(
                            f'<summary>Logs <span class="line-count">'
                            f"{log_lines} lines</span></summary>\n"
                        )
                        parts.append(
                            f'<div class="collapsible-content">'
                            f"<pre><code>{escaped_log}</code></pre></div>\n"
                        )
                        parts.append("</details>\n")

                    # Previous logs
                    prev_log = os.path.join(pods_dir, f"{pod_name}-previous.log")
                    if os.path.isfile(prev_log) and os.path.getsize(prev_log) > 0:
                        prev_content = read_file(prev_log)
                        escaped_prev = highlight_logs(html.escape(prev_content))
                        prev_lines = len(prev_content.split("\n"))
                        parts.append('<details class="collapsible">\n')
                        parts.append(
                            f'<summary>Previous Logs <span class="line-count">'
                            f"{prev_lines} lines</span></summary>\n"
                        )
                        parts.append(
                            f'<div class="collapsible-content">'
                            f"<pre><code>{escaped_prev}</code></pre></div>\n"
                        )
                        parts.append("</details>\n")

                    parts.append("</div>\n")

            parts.append("</div></details>\n")

    parts.append("""\
</div>
</div>
""")
    return "".join(parts)


def render_diagnostics(report_dir):
    """Render the OFED diagnostics section."""
    diag_dir = os.path.join(
        report_dir, "operator", "components", "ofed-driver", "diagnostics"
    )

    parts = []
    parts.append("""\
<div class="section" id="diagnostics">
<div class="section-header">
    <h2>OFED Diagnostics</h2>
    <div class="expand-controls">
        <button onclick="toggleAll(this,'diagnostics',true)">Expand All</button>
        <button onclick="toggleAll(this,'diagnostics',false)">Collapse All</button>
    </div>
</div>
<div class="section-body">
""")

    if not os.path.isdir(diag_dir) or not os.listdir(diag_dir):
        parts.append(
            '<div class="empty-state">No OFED diagnostics collected '
            "(--skip-diagnostics may have been used)</div>\n"
        )
    else:
        # Group files by node name
        known_commands = [
            "lsmod", "ibstat", "ibv_devinfo", "mst_status",
            "kernel_version", "dmesg", "ip_link", "ip_addr",
        ]
        seen_nodes = []
        seen_set = set()

        for f in sorted(glob.glob(os.path.join(diag_dir, "*.txt"))):
            fname = os.path.splitext(os.path.basename(f))[0]
            node_name = ""
            for cmd in known_commands:
                suffix = f"-{cmd}"
                if fname.endswith(suffix):
                    node_name = fname[: -len(suffix)]
                    break
            if not node_name:
                continue
            if node_name not in seen_set:
                seen_nodes.append(node_name)
                seen_set.add(node_name)

        if not seen_nodes:
            parts.append('<div class="empty-state">No diagnostic files found</div>\n')
        else:
            for node in seen_nodes:
                parts.append('<div class="node-card">\n')
                parts.append(
                    f'<div class="node-card-header">{html.escape(node)}</div>\n'
                )
                parts.append('<div class="node-card-body">\n')

                # Kernel version (inline)
                kver_file = os.path.join(diag_dir, f"{node}-kernel_version.txt")
                if os.path.isfile(kver_file):
                    kver = html.escape(read_file(kver_file).split("\n")[0])
                    parts.append(
                        f"<p><strong>Kernel:</strong> <code>{kver}</code></p>\n"
                    )

                # Each diagnostic command as a collapsible
                display_commands = [
                    "lsmod", "ibstat", "ibv_devinfo", "mst_status",
                    "dmesg", "ip_link", "ip_addr",
                ]
                display_names = {
                    "lsmod": "lsmod | grep mlx",
                    "mst_status": "mst status",
                    "kernel_version": "Kernel Version",
                    "ip_link": "ip link show",
                    "ip_addr": "ip addr show",
                }
                for cmd in display_commands:
                    cmd_file = os.path.join(diag_dir, f"{node}-{cmd}.txt")
                    if os.path.isfile(cmd_file) and os.path.getsize(cmd_file) > 0:
                        display_cmd = display_names.get(cmd, cmd)
                        parts.append(collapsible(
                            html.escape(display_cmd),
                            file_content_escaped(cmd_file),
                            "yaml-block",
                        ))

                parts.append("</div></div>\n")

    parts.append("""\
</div>
</div>
""")
    return "".join(parts)


def render_nodes(report_dir):
    """Render the node overview section."""
    parts = []
    parts.append("""\
<div class="section" id="nodes">
<div class="section-header">
    <h2>Node Overview</h2>
    <div class="expand-controls">
        <button onclick="toggleAll(this,'nodes',true)">Expand All</button>
        <button onclick="toggleAll(this,'nodes',false)">Collapse All</button>
    </div>
</div>
<div class="section-body">
""")

    nodes_dir = os.path.join(report_dir, "nodes")
    if not os.path.isdir(nodes_dir):
        parts.append('<div class="empty-state">No node data found</div>\n')
    else:
        # Node summary table
        summary_file = os.path.join(nodes_dir, "nodes-summary.txt")
        if os.path.isfile(summary_file) and os.path.getsize(summary_file) > 0:
            parts.append('<h4 style="margin-bottom:8px;">Node Summary</h4>\n')
            parts.append('<div style="overflow-x:auto;">\n')
            parts.append(
                '<pre style="background:var(--code-bg);color:var(--code-text);'
                'padding:12px;border-radius:6px;font-size:12px;">\n'
            )
            parts.append(html.escape(read_file(summary_file)))
            parts.append("</pre></div>\n")

        # Node resources
        resources_file = os.path.join(nodes_dir, "node-resources.txt")
        if os.path.isfile(resources_file) and os.path.getsize(resources_file) > 0:
            parts.append(collapsible(
                "Node Resources (RDMA, SR-IOV, GPU)",
                file_content_escaped(resources_file),
                "",
            ))

        # Node labels
        labels_file = os.path.join(nodes_dir, "node-labels.txt")
        if os.path.isfile(labels_file) and os.path.getsize(labels_file) > 0:
            parts.append(collapsible(
                "Node Labels",
                file_content_escaped(labels_file),
                "",
            ))

        # Full node YAML
        all_nodes_file = os.path.join(nodes_dir, "all-nodes.yaml")
        if os.path.isfile(all_nodes_file) and os.path.getsize(all_nodes_file) > 0:
            parts.append(collapsible(
                "All Nodes Full YAML",
                file_content_escaped(all_nodes_file),
                "yaml-block",
            ))

    parts.append("""\
</div>
</div>
""")
    return "".join(parts)


def render_events(report_dir):
    """Render the events section."""
    events_file = os.path.join(report_dir, "operator", "events.yaml")

    parts = []
    parts.append("""\
<div class="section" id="events">
<div class="section-header">
    <h2>Events</h2>
</div>
<div class="section-body">
""")

    if not os.path.isfile(events_file) or os.path.getsize(events_file) == 0:
        parts.append('<div class="empty-state">No events collected</div>\n')
    else:
        events_content = read_file(events_file)

        # Parse events using a state-machine approach (like the awk in bash)
        ev_type = ""
        reason = ""
        message = ""
        timestamp = ""
        obj = ""
        obj_set = False

        def emit_event():
            if not reason:
                return ""
            css_class = "event-item"
            if ev_type == "Warning":
                css_class = "event-item event-warning"
            out = f'<div class="{css_class}">\n'
            out += (
                f'<div class="event-time">'
                f"{html.escape(timestamp)} &nbsp; {html.escape(obj)}</div>\n"
            )
            out += f'<div><span class="event-reason">{html.escape(reason)}</span>'
            if ev_type == "Warning":
                out += f" {status_badge('Warning')}"
            out += "</div>\n"
            out += f"<div>{html.escape(message)}</div>\n"
            out += "</div>\n"
            return out

        for line in events_content.split("\n"):
            if line.startswith("- apiVersion:") or line.startswith("---"):
                parts.append(emit_event())
                ev_type = ""
                reason = ""
                message = ""
                timestamp = ""
                obj = ""
                obj_set = False
                continue

            m = re.match(r"^  type:\s*(\S+)", line)
            if m:
                ev_type = m.group(1)
                continue

            m = re.match(r"^  reason:\s*(\S+)", line)
            if m:
                reason = m.group(1)
                continue

            m = re.match(r"^  message:\s*(.*)", line)
            if m:
                message = m.group(1).strip()
                continue

            m = re.match(r"^  lastTimestamp:\s*(\S+)", line)
            if m:
                timestamp = m.group(1)
                continue

            m = re.match(r"^    name:\s*(\S+)", line)
            if m and not obj_set:
                obj = m.group(1)
                obj_set = True
                continue

        # Emit last event
        parts.append(emit_event())

        # Also provide full YAML
        parts.append(collapsible(
            "Events Full YAML",
            file_content_escaped(events_file),
            "yaml-block",
        ))

    parts.append("""\
</div>
</div>
""")
    return "".join(parts)


def render_crds(report_dir):
    """Render the CRD inventory section."""
    parts = []
    parts.append("""\
<div class="section" id="crds">
<div class="section-header">
    <h2>CRD Inventory</h2>
    <div class="expand-controls">
        <button onclick="toggleAll(this,'crds',true)">Expand All</button>
        <button onclick="toggleAll(this,'crds',false)">Collapse All</button>
    </div>
</div>
<div class="section-body">
""")

    # CRD Definitions
    defs_dir = os.path.join(report_dir, "crds", "definitions")
    if os.path.isdir(defs_dir) and os.listdir(defs_dir):
        def_files = sorted(glob.glob(os.path.join(defs_dir, "*.yaml")))
        def_count = len(def_files)
        parts.append(
            f'<h4 style="margin-bottom:8px;">CRD Definitions '
            f'<span style="color:var(--text-secondary);font-weight:400;'
            f'font-size:13px;">({def_count})</span></h4>\n'
        )
        for crd_file in def_files:
            crd_name = os.path.splitext(os.path.basename(crd_file))[0]
            parts.append('<details class="crd-row">\n')
            parts.append('<summary class="crd-row-summary">\n')
            parts.append('<span class="expand-chevron">&#9654;</span>\n')
            parts.append(f'<span class="crd-name">{html.escape(crd_name)}</span>\n')
            parts.append("</summary>\n")
            parts.append('<div class="crd-row-detail">\n')
            parts.append(
                f'<pre class="yaml-block"><code>'
                f"{html.escape(read_file(crd_file))}</code></pre>\n"
            )
            parts.append("</div></details>\n")
    else:
        parts.append('<p class="empty-state">No CRD definitions collected</p>\n')

    # CRD Instances
    inst_dir = os.path.join(report_dir, "crds", "instances")
    if os.path.isdir(inst_dir) and os.listdir(inst_dir):
        type_dirs = sorted_dirs(inst_dir)
        inst_type_count = len(type_dirs)
        parts.append(
            f'<h4 style="margin:16px 0 8px;">CR Instances '
            f'<span style="color:var(--text-secondary);font-weight:400;'
            f'font-size:13px;">({inst_type_count} types)</span></h4>\n'
        )

        for type_path in type_dirs:
            type_name = os.path.basename(type_path)
            all_yaml = os.path.join(type_path, "all.yaml")
            inst_count = 0
            if os.path.isfile(all_yaml):
                content = read_file(all_yaml)
                inst_count = len(re.findall(
                    r"^-\s*apiVersion:|^apiVersion:", content, re.MULTILINE
                ))
                if inst_count == 0:
                    inst_count = 1

            plural = "s" if inst_count != 1 else ""
            parts.append('<details class="crd-row">\n')
            parts.append('<summary class="crd-row-summary">\n')
            parts.append('<span class="expand-chevron">&#9654;</span>\n')
            parts.append(
                f'<span class="crd-name">{html.escape(type_name)}</span>\n'
            )
            parts.append(
                f'<span class="crd-count">'
                f"{inst_count} resource{plural}</span>\n"
            )
            parts.append("</summary>\n")
            if os.path.isfile(all_yaml) and os.path.getsize(all_yaml) > 0:
                parts.append('<div class="crd-row-detail">\n')
                parts.append(
                    f'<pre class="yaml-block"><code>'
                    f"{html.escape(read_file(all_yaml))}</code></pre>\n"
                )
                parts.append("</div>\n")
            parts.append("</details>\n")
    else:
        parts.append('<p class="empty-state">No CR instances collected</p>\n')

    parts.append("""\
</div>
</div>
""")
    return "".join(parts)


def render_rbac(report_dir):
    """Render the RBAC section."""
    rbac_dir = os.path.join(report_dir, "operator", "rbac")

    parts = []
    parts.append("""\
<div class="section" id="rbac">
<div class="section-header">
    <h2>RBAC</h2>
    <div class="expand-controls">
        <button onclick="toggleAll(this,'rbac',true)">Expand All</button>
        <button onclick="toggleAll(this,'rbac',false)">Collapse All</button>
    </div>
</div>
<div class="section-body">
""")

    if not os.path.isdir(rbac_dir) or not os.listdir(rbac_dir):
        parts.append('<div class="empty-state">No RBAC data collected</div>\n')
    else:
        for rbac_file in sorted(glob.glob(os.path.join(rbac_dir, "*.yaml"))):
            title = os.path.splitext(os.path.basename(rbac_file))[0]
            parts.append(collapsible(
                html.escape(title),
                file_content_escaped(rbac_file),
                "yaml-block",
            ))

    parts.append("""\
</div>
</div>
""")
    return "".join(parts)


def render_network(report_dir):
    """Render the network and webhooks section."""
    parts = []
    parts.append("""\
<div class="section" id="network">
<div class="section-header">
    <h2>Network &amp; Webhooks</h2>
    <div class="expand-controls">
        <button onclick="toggleAll(this,'network',true)">Expand All</button>
        <button onclick="toggleAll(this,'network',false)">Collapse All</button>
    </div>
</div>
<div class="section-body">
""")

    has_content = False

    # Services
    services_file = os.path.join(report_dir, "network", "services.yaml")
    if os.path.isfile(services_file) and os.path.getsize(services_file) > 0:
        parts.append(collapsible(
            "Services",
            file_content_escaped(services_file),
            "yaml-block",
        ))
        has_content = True

    # Webhooks
    for wh in [
        "validatingwebhookconfigurations.yaml",
        "mutatingwebhookconfigurations.yaml",
    ]:
        wh_file = os.path.join(report_dir, "operator", wh)
        if os.path.isfile(wh_file) and os.path.getsize(wh_file) > 0:
            title = os.path.splitext(os.path.basename(wh_file))[0]
            parts.append(collapsible(
                html.escape(title),
                file_content_escaped(wh_file),
                "yaml-block",
            ))
            has_content = True

    if not has_content:
        parts.append(
            '<div class="empty-state">No network/webhook data collected</div>\n'
        )

    parts.append("""\
</div>
</div>
""")
    return "".join(parts)


def render_config(report_dir):
    """Render the operator configuration section."""
    parts = []
    parts.append("""\
<div class="section" id="config">
<div class="section-header">
    <h2>Operator Configuration</h2>
    <div class="expand-controls">
        <button onclick="toggleAll(this,'config',true)">Expand All</button>
        <button onclick="toggleAll(this,'config',false)">Collapse All</button>
    </div>
</div>
<div class="section-body">
""")

    has_content = False

    # ConfigMaps
    cm_file = os.path.join(report_dir, "operator", "configmaps.yaml")
    if os.path.isfile(cm_file) and os.path.getsize(cm_file) > 0:
        parts.append(collapsible(
            "ConfigMaps",
            file_content_escaped(cm_file),
            "yaml-block",
        ))
        has_content = True

    # Secrets metadata
    secrets_file = os.path.join(report_dir, "operator", "secrets-metadata.txt")
    if os.path.isfile(secrets_file) and os.path.getsize(secrets_file) > 0:
        parts.append(collapsible(
            "Secrets (metadata only)",
            file_content_escaped(secrets_file),
            "",
        ))
        has_content = True

    # Namespace
    ns_file = os.path.join(report_dir, "operator", "namespace.yaml")
    if os.path.isfile(ns_file) and os.path.getsize(ns_file) > 0:
        parts.append(collapsible(
            "Namespace",
            file_content_escaped(ns_file),
            "yaml-block",
        ))
        has_content = True

    if not has_content:
        parts.append(
            '<div class="empty-state">No configuration data collected</div>\n'
        )

    parts.append("""\
</div>
</div>
""")
    return "".join(parts)


def render_related_operators(report_dir):
    """Render the related operators section."""
    related_dir = os.path.join(report_dir, "related-operators")

    parts = []
    parts.append("""\
<div class="section" id="related">
<div class="section-header">
    <h2>Related Operators</h2>
    <div class="expand-controls">
        <button onclick="toggleAll(this,'related',true)">Expand All</button>
        <button onclick="toggleAll(this,'related',false)">Collapse All</button>
    </div>
</div>
<div class="section-body">
""")

    if not os.path.isdir(related_dir) or not os.listdir(related_dir):
        parts.append(
            '<div class="empty-state">No related operators collected</div>\n'
        )
    else:
        for op_path in sorted_dirs(related_dir):
            op_name = os.path.basename(op_path)
            parts.append(f"<h4>{html.escape(op_name)}</h4>\n")
            # Collect .yaml and .txt files
            files = sorted(
                glob.glob(os.path.join(op_path, "*.yaml"))
                + glob.glob(os.path.join(op_path, "*.txt"))
            )
            for f in files:
                if not os.path.isfile(f) or os.path.getsize(f) == 0:
                    continue
                parts.append(collapsible(
                    html.escape(os.path.basename(f)),
                    file_content_escaped(f),
                    "yaml-block",
                ))

    parts.append("""\
</div>
</div>
""")
    return "".join(parts)


def render_metadata(report_dir):
    """Render the cluster metadata section."""
    parts = []
    parts.append("""\
<div class="section" id="metadata">
<div class="section-header">
    <h2>Cluster Metadata</h2>
    <div class="expand-controls">
        <button onclick="toggleAll(this,'metadata',true)">Expand All</button>
        <button onclick="toggleAll(this,'metadata',false)">Collapse All</button>
    </div>
</div>
<div class="section-body">
""")

    meta_dir = os.path.join(report_dir, "metadata")
    has_content = False

    if os.path.isdir(meta_dir):
        ordered_files = [
            "collection-info.txt",
            "cluster-version.yaml",
            "namespaces.txt",
            "api-resources.txt",
        ]
        for fname in ordered_files:
            fpath = os.path.join(meta_dir, fname)
            if os.path.isfile(fpath) and os.path.getsize(fpath) > 0:
                parts.append(collapsible(
                    html.escape(fname),
                    file_content_escaped(fpath),
                    "yaml-block",
                ))
                has_content = True

    if not has_content:
        parts.append('<div class="empty-state">No metadata collected</div>\n')

    parts.append("""\
</div>
</div>
""")
    return "".join(parts)


def render_errors(report_dir):
    """Render the collection errors and warnings section."""
    error_log = os.path.join(report_dir, "collection-errors.log")

    parts = []
    parts.append("""\
<div class="section" id="errors">
<div class="section-header">
    <h2>Collection Errors &amp; Warnings</h2>
</div>
<div class="section-body">
""")

    if not os.path.isfile(error_log) or os.path.getsize(error_log) == 0:
        parts.append(
            '<div class="empty-state" style="color:var(--status-ready);">'
            "No errors or warnings during collection</div>\n"
        )
    else:
        content = read_file(error_log)
        err_count = len(re.findall(r"\[ERROR\]", content))
        warn_count = len(re.findall(r"\[WARN\]", content))
        parts.append(
            f"<p><strong>{err_count}</strong> errors, "
            f"<strong>{warn_count}</strong> warnings during collection</p>\n"
        )

        escaped_errors = html.escape(content)
        # Highlight error and warning lines
        highlighted_lines = []
        for line in escaped_errors.split("\n"):
            if "[ERROR]" in line:
                highlighted_lines.append(f'<span class="log-error">{line}</span>')
            elif "[WARN]" in line:
                highlighted_lines.append(f'<span class="log-warn">{line}</span>')
            else:
                highlighted_lines.append(line)
        escaped_errors = "\n".join(highlighted_lines)

        total_lines = len(content.split("\n"))
        parts.append('<details class="collapsible" open>\n')
        parts.append(
            f'<summary>Error Log <span class="line-count">'
            f"{total_lines} lines</span></summary>\n"
        )
        parts.append(
            f'<div class="collapsible-content">'
            f"<pre><code>{escaped_errors}</code></pre></div>\n"
        )
        parts.append("</details>\n")

    parts.append("""\
</div>
</div>
""")
    return "".join(parts)


# ============================================================================
# Main
# ============================================================================

def main():
    parser = argparse.ArgumentParser(
        description="NVIDIA Network Operator SOS-Report HTML Report Generator",
        epilog=(
            "Examples:\n"
            "  %(prog)s ./network-operator-sosreport-20260218-143000/\n"
            "  %(prog)s ./network-operator-sosreport-20260218-143000/ "
            "--output /tmp/report.html\n"
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "report_dir",
        help="Path to the sosreport collection directory",
    )
    parser.add_argument(
        "--output",
        default=None,
        help="Output HTML file path (default: <report_dir>/report.html)",
    )
    parser.add_argument(
        "--template",
        default=None,
        help=(
            "Path to the HTML template file "
            "(default: report-template.html in the same directory as this script)"
        ),
    )

    args = parser.parse_args()

    # Normalize report directory
    report_dir = args.report_dir.rstrip("/")

    if not os.path.isdir(report_dir):
        print(f"Error: Directory not found: {report_dir}", file=sys.stderr)
        sys.exit(1)

    # Determine output file
    output_file = args.output if args.output else os.path.join(report_dir, "report.html")

    # Find template file
    template_path = args.template
    if not template_path:
        script_dir = os.path.dirname(os.path.abspath(__file__))
        template_path = os.path.join(script_dir, "report-template.html")

    if not os.path.isfile(template_path):
        print(f"Error: Template file not found: {template_path}", file=sys.stderr)
        sys.exit(1)

    # Read the template
    template_content = read_file(template_path)
    if not template_content:
        print(f"Error: Template file is empty: {template_path}", file=sys.stderr)
        sys.exit(1)

    # Render all sections
    sections = {
        "SECTION_DASHBOARD": render_dashboard(report_dir),
        "SECTION_NCP_STATUS": render_ncp_status(report_dir),
        "SECTION_COMPONENTS": render_components(report_dir),
        "SECTION_DIAGNOSTICS": render_diagnostics(report_dir),
        "SECTION_NODES": render_nodes(report_dir),
        "SECTION_EVENTS": render_events(report_dir),
        "SECTION_METADATA": render_metadata(report_dir),
        "SECTION_CRDS": render_crds(report_dir),
        "SECTION_RBAC": render_rbac(report_dir),
        "SECTION_NETWORK": render_network(report_dir),
        "SECTION_CONFIG": render_config(report_dir),
        "SECTION_RELATED_OPERATORS": render_related_operators(report_dir),
        "SECTION_ERRORS": render_errors(report_dir),
    }

    # Substitute sections into template
    tmpl = string.Template(template_content)
    output_content = tmpl.safe_substitute(sections)

    # Write output
    with open(output_file, "w", encoding="utf-8") as f:
        f.write(output_content)

    print(f"HTML report generated: {output_file}")


if __name__ == "__main__":
    main()
