import scriptling.mcp.tool as tool

period = tool.get_string("period")
region = tool.get_string("region", "all")

tool.return_object({
    "messages": [
        {
            "role": "user",
            "content": "Review the sales for " + period + " (region: " + region + "). "
                       "Call sales_report for the records and highlight the single largest sale.",
        }
    ]
})
