> For the complete documentation index, see [llms.txt](https://docs.n8n.io/llms.txt). Markdown versions of documentation pages are available by appending `.md` to page URLs; this page is available as [Markdown](https://docs.n8n.io/build/understand-workflows/understand-executions/view-executions-for-a-single-workflow.md).

# View executions for a single workflow

View and filter all executions for the workflow currently open on the canvas.

The **Executions** list in a workflow shows all executions for that workflow.

{% hint style="info" %}
**Deleted workflows**

When you delete a workflow, n8n deletes its execution history as well. This means you can't view executions for deleted workflows.
{% endhint %}

{% hint style="info" %}
**Execution history and workflow history**

Don't confuse the execution list with [Workflow history](/build/manage-workflows/view-change-history.md).

Executions are workflow runs. With the executions list, you can see previous runs of the current version of the workflow. You can copy previous executions into the editor to [Debug and re-run past executions](/build/understand-workflows/understand-executions/debug-executions.md) in your current workflow.

Workflow history is previous versions of the workflow: for example, a version with a different node, or different parameters set.
{% endhint %}

## View executions for a single workflow <a href="#view-executions-for-a-single-workflow" id="view-executions-for-a-single-workflow"></a>

In the workflow, select the **Executions** tab in the top menu. You can preview all executions of that workflow.

{% hint style="info" %}
**Executions using end-user credentials**

When an execution uses an [end-user credential](https://app.gitbook.com/s/wMJrGrimpx3PxCJpUswm/manage-credentials/end-user-credentials), only the user whose connected account ran the node can see that node's data. For everyone else, including admins, the output is redacted.
{% endhint %}

## Filter executions <a href="#filter-executions" id="filter-executions"></a>

You can filter the executions list.

1. In your workflow, select **Executions**.
2. Select **Filters**.
3. Enter your filters. You can filter by:
   * **Status**: choose from **Failed**, **Running**, **Success**, or **Waiting**.
   * **Execution start**: see executions that started in the given time.
   * **Saved custom data**: this is data you create within the workflow using the Code node. Enter the key and value to filter. Refer to [Custom executions data](/build/understand-workflows/understand-executions/customize-executions-data.md) for information on adding custom data.

{% hint style="info" %}
**Feature availability**

Custom executions data is available on:

* Cloud: Pro, Enterprise
* Self-Hosted: Enterprise, registered Community
  {% endhint %}

## Retry failed workflows <a href="#retry-failed-workflows" id="retry-failed-workflows"></a>

If your workflow execution fails, you can retry the execution. To retry a failed workflow:

1. Open the **Executions** list.
2. For the workflow execution you want to retry, select **Refresh** <img src="https://3488703508-files.gitbook.io/~/files/v0/b/gitbook-x-prod.appspot.com/o/spaces%2FrPN1zU5jaYNvwH7RzxqA%2Fuploads%2Fgit-blob-f422242e07871bf736b704bb4a47df45b43dbd70%2Frefresh.png?alt=media" alt="Refresh icon" data-size="line">.
3. Select either of the following options to retry the execution:
   * **Retry with currently saved workflow**: Once you make changes to your workflow, you can select this option to execute the workflow with the previous execution data.
   * **Retry with original workflow**: If you want to retry the execution without making changes to your workflow, you can select this option to retry the execution with the previous execution data.
