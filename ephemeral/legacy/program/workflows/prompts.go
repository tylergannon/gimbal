package workflows

import (
	"fmt"

	"github.com/tylergannon/gimbal/program"
)

const (
	sprintImplementPrompt   = "Execute the current sprint. Work through its document phase by phase, in order, and prove each phase before moving on. Commit as you go with specific paths. If something blocks you, record what is blocked and why, then continue with the next unblocked task."
	sprintReviewPrompt      = "Review the work against the current sprint's document. Run the software yourself; do not judge from the diff alone. Name any specific material defect that needs repair. The engine runs the item's check and owns done."
	chapterImplementPrompt  = "Execute the current sprint, keeping the work aligned with the chapter it belongs to and inside that chapter's non-goals."
	chapterReviewPrompt     = "Review the work against the current sprint's document and the chapter it serves. Run the software yourself. Name any specific material defect that needs repair. The engine runs the item's check and owns done."
	deliveryImplementPrompt = "Do the current plan item. Test what you write, run the repository's standard test set, and commit with specific paths."
	deliveryReviewPrompt    = "Review the current item against the specification. Run the software and the repository's test set yourself rather than reading the diff. Name any specific material defect that needs repair. The engine runs the item's check and owns done."
)

func itemPrompt(goal string, iteration program.Iteration, instruction string) string {
	return fmt.Sprintf("%s\n\nRun goal: %s\n\nCurrent checklist item:\n%s\n\nFeedback from the previous loop arrival:\n%s", instruction, goal, iteration.Item.Render(), iteration.Feedback)
}

func planPrompt(goal, path string) string {
	return fmt.Sprintf("%s\n\nRead the specification named above, do focused recon of this workspace, and write %s as a checklist: YAML frontmatter with one item per unit of work, each with a name, the check that proves it, and the command that runs the check. Put the definition of done in prose below the frontmatter. Do not write a done field on any item.", goal, path)
}
func critiquePrompt(goal, path string) string {
	return fmt.Sprintf("%s\n\nRead that specification and %s. Write plan-critique.md: where the plan is wrong, where it is missing work, where an item's check would pass while the item is false, and where it is stricter than the specification requires.", goal, path)
}
func updatePrompt(goal, path string) string {
	return fmt.Sprintf("%s\n\nRead that specification, %s, and plan-critique.md. Rewrite %s, taking only changes that reduce risk or make the work provable. Keep the checklist shape and write no done fields.", goal, path, path)
}

func implementationPrompt(run ledgerRun, iteration program.Iteration) string {
	return itemPrompt(run.Goal, iteration, run.ImplementPrompt+"\n\n"+run.WorkContext)
}

func repairPrompt(run ledgerRun, iteration program.Iteration, notes string) string {
	return itemPrompt(run.Goal, iteration, run.ImplementPrompt+"\n\n"+run.WorkContext+"\n\nRepair this material defect before review again:\n"+notes)
}

func reviewPrompt(instruction, workContext string) string {
	return instruction + "\n\n" + workContext + "\n\nReturn material_defect true only when you found a specific defect that requires another implementation turn. If work is ready for the engine check, or that check is needed to settle uncertainty, return false."
}

func evaluationPrompt(subject, goal, body string) string {
	return fmt.Sprintf("Evaluate whether this %s is complete. Goal: %s\n\nDefinition of done:\n%s\n\nAll items have passed their mechanical validation. Decide whether that is sufficient.", subject, goal, body)
}

func chapterContext(chapter program.Iteration) string {
	return "Current chapter ledger item:\n" + chapter.Item.Render() + "\n\nChapter loop feedback:\n" + chapter.Feedback
}
