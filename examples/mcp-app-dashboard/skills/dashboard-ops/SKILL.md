---
name: dashboard-ops
description: Operating the sales dashboard app — reading reports, adding sales, and interpreting the chart
version: 1.0.0
---

# Dashboard Operations

The sales dashboard is an MCP Apps view: call `sales_report` and the host
renders the interactive dashboard instead of plain text.

## Reading the report

Call `sales_report` with no arguments. It returns the current sales records;
the dashboard renders them as a table and a Chart.js chart.

## Adding a sale

The dashboard's "Add Sale" form calls `add_sale` itself (the tool is
app-visible only). Only call it directly when scripting without the view.

## Interpreting results

Revenue is grouped by region. When a user asks "which region is ahead",
read the table rather than recomputing from raw records — the tool result
is the source of truth.
