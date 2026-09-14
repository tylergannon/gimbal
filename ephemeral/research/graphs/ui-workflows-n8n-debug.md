> For the complete documentation index, see [llms.txt](https://docs.n8n.io/llms.txt). Markdown versions of documentation pages are available by appending `.md` to page URLs; this page is available as [Markdown](https://docs.n8n.io/build/understand-workflows/understand-executions/debug-executions.md).

# Debug executions

How to copy execution data into your current workflow in order to debug previous executions.

{% hint style="info" %}
**Feature availability**

Debugging and re-running past executions is available on:

* **n8n Cloud:** All plans
* **Self-hosted:** Registered Community, Business, Enterprise
  {% endhint %}

You can load data from a previous execution into your current workflow. This is useful for debugging data from failed production executions: you can see a failed execution, make changes to your workflow to fix it, then re-run it with the previous execution data.

## Load data <a href="#load-data" id="load-data"></a>

To load data from a previous execution:

1. In your workflow, select the **Executions** tab to view the **Executions** list.
2. Select the execution you want to debug. n8n displays options depending on whether the workflow was successful or failed:
   * For failed executions: select **Debug in editor**.
   * For successful executions: select **Copy to editor**.
3. n8n copies the execution data into your current workflow, and [pins the data](/build/work-with-data/pin-and-mock-data.md) in the first node in the workflow.

{% hint style="info" %}
**Check which executions you save**

The executions available on the **Executions** list depends on your [Workflow settings](/build/manage-workflows/configure-workflow-settings.md).
{% endhint %}
