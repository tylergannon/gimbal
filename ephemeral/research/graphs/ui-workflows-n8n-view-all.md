> For the complete documentation index, see [llms.txt](https://docs.n8n.io/llms.txt). Markdown versions of documentation pages are available by appending `.md` to page URLs; this page is available as [Markdown](https://docs.n8n.io/build/understand-workflows/understand-executions/view-all-executions.md).

# View all executions

View and filter all executions for all workflows.

To view **all executions** from an n8n instance, navigate to the **Overview** page and then click into the Executions tab. This will show you all executions from the workflows you have access to.

If your n8n instance supports **projects**, you'll also be able to view the executions tab within projects you have access to. This will show you executions only from the workflows within the specified project.

{% hint style="info" %}
**Deleted workflows**

When you delete a workflow, n8n deletes its execution history as well. This means you can't view executions for deleted workflows.
{% endhint %}

## Filter executions <a href="#filter-executions" id="filter-executions"></a>

You can filter the executions list:

1. Select the **Executions** tab either from within the **Overview** page or a specific **project** to open the list.
2. Select **Filters**.
3. Enter your filters. You can filter by:
   * **Workflows**: choose all workflows, or a specific workflow name.
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

1. Select the **Executions** tab from within either the **Overview** page or a specific **project** to open the list.
2. On the execution you want to retry, select **Retry execution** <img src="https://3488703508-files.gitbook.io/~/files/v0/b/gitbook-x-prod.appspot.com/o/spaces%2FrPN1zU5jaYNvwH7RzxqA%2Fuploads%2Fgit-blob-f19a4d097d364587b73864eaed77b12bfd8f516b%2Fthree-dot-options-menu.png?alt=media" alt="Options menu icon" data-size="line">.
3. Select either of the following options to retry the execution:
   * **Retry with currently saved workflow**: Once you make changes to your workflow, you can select this option to execute the workflow with the previous execution data.
   * **Retry with original workflow**: If you want to retry the execution without making changes to your workflow, you can select this option to retry the execution with the previous execution data.

## Load data from previous executions into your current workflow <a href="#load-data-from-previous-executions-into-your-current-workflow" id="load-data-from-previous-executions-into-your-current-workflow"></a>

You can load data from a previous workflow back into the canvas. Refer to [Debug executions](/build/understand-workflows/understand-executions/debug-executions.md) for more information.
