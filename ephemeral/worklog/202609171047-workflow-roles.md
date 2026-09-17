correction: Workflow roles are typed cognitive skills, named as RoleCodeReview, rather than raw job-title strings.
decision: Maintain prescribed WorkflowRole constants and their 10-20-word descriptions together in root roles.go.
decision: Consumer applications may define additional WorkflowRole constants; the built-in catalog remains open rather than an enum.
