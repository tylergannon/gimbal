package gimble

// WorkflowRole names the cognitive work a session performs and selects its
// model binding for a run. Applications may define additional typed constants.
type WorkflowRole string

const (
	// RoleCSMathHardProblems solves difficult computer science and mathematics problems requiring rigorous multi-step reasoning and verification.
	RoleCSMathHardProblems WorkflowRole = "cs-math-hard-problems"

	// RoleImageComprehension interprets images, diagrams, screenshots, and visual relationships to produce accurate structured understanding.
	RoleImageComprehension WorkflowRole = "image-comprehension"

	// RoleFrontendAesthetics judges and improves visual polish, composition, typography, spacing, color, and interaction feel.
	RoleFrontendAesthetics WorkflowRole = "frontend-aesthetics"

	// RoleFrontendArchitecture designs maintainable frontend boundaries, state flow, rendering strategy, component ownership, and integration seams.
	RoleFrontendArchitecture WorkflowRole = "frontend-architecture"

	// RoleArchitecturalCritique finds structural risks, unclear boundaries, hidden coupling, and unnecessary complexity in proposed system designs.
	RoleArchitecturalCritique WorkflowRole = "architectural-critique"

	// RoleSprintPlanning turns an accepted outcome into bounded, ordered work with explicit dependencies and validation.
	RoleSprintPlanning WorkflowRole = "sprint-planning"

	// RoleDevOpsTasks handles deployment, infrastructure, automation, observability, and operational troubleshooting across development and production environments.
	RoleDevOpsTasks WorkflowRole = "devops-tasks"

	// RoleQAOrchestration coordinates test strategy, execution, evidence collection, and risk-based validation across a delivery effort.
	RoleQAOrchestration WorkflowRole = "qa-orchestration"

	// RoleSecurityReview identifies exploitable weaknesses, trust-boundary failures, unsafe defaults, and missing defenses using concrete attack paths.
	RoleSecurityReview WorkflowRole = "security-review"

	// RoleBulkClassification classifies many independent items consistently against a defined taxonomy, rubric, or decision boundary.
	RoleBulkClassification WorkflowRole = "bulk-classification"

	// RoleBulkMapReduce processes large collections independently, then combines partial results into a coherent, checked synthesis.
	RoleBulkMapReduce WorkflowRole = "bulk-map-reduce"

	// RoleUXIdeation generates and compares user experience concepts grounded in audience needs, constraints, and desired outcomes.
	RoleUXIdeation WorkflowRole = "ux-ideation"

	// RoleCopyWriting writes clear, audience-aware copy with appropriate voice, structure, persuasion, and factual discipline.
	RoleCopyWriting WorkflowRole = "copy-writing"

	// RoleAgenticDialogues conducts ongoing user conversations while choosing tools, preserving context, and advancing the user's intent.
	RoleAgenticDialogues WorkflowRole = "agentic-dialogues"

	// RoleVoiceInteractive handles low-latency spoken interaction with natural turn-taking, concise responses, and reliable intent tracking.
	RoleVoiceInteractive WorkflowRole = "voice-interactive"

	// RoleCodeReview finds concrete correctness defects in code and reports only actionable, evidence-backed findings.
	RoleCodeReview WorkflowRole = "code-review"
)
