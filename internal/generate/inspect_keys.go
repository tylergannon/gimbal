package generate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type presentationFingerprint struct {
	File       string
	Line       int
	Column     int
	Scope      string
	Kind       string
	Label      string
	Header     string
	Expression string
	Session    string
	From       string
	Prompt     string
}

type presentationCandidate struct {
	token string
	set   func(string)
}

// assignPresentationKeys attaches content/source-site keys independently of
// viewNode.ID, which remains a sequential layout identifier. Identical tokens
// are intentionally omitted: without another exact identity bit, choosing one
// duplicate would make persisted state attach to an arbitrary operation.
func assignPresentationKeys(nodes []*viewNode) {
	var candidates []presentationCandidate
	var collect func([]*viewNode)
	collect = func(nodes []*viewNode) {
		for _, n := range nodes {
			n.Key = ""
			fingerprint := presentationFingerprint{
				File: n.Source.File, Line: n.Source.Line, Scope: n.Scope,
				Kind: n.Kind, Label: n.Label, Session: n.Session,
				From: n.From, Prompt: n.Prompt,
			}
			if n.Control != nil {
				fingerprint.Header = n.Control.Code
			}
			if n.Detail != nil {
				fingerprint.Expression = n.Detail.Expression + "\x00" +
					n.Detail.PromptExpression + "\x00" + n.Detail.KeyExpression + "\x00" +
					n.Detail.ValueExpression + "\x00" + n.Detail.ContextExpression
			}
			candidates = append(candidates, presentationCandidate{
				token: presentationToken(fingerprint),
				set:   func(key string) { n.Key = key },
			})

			for i := range n.Branches {
				branch := &n.Branches[i]
				branch.Key = ""
				label := ""
				if i < len(n.BranchLabels) {
					label = n.BranchLabels[i]
				}
				branchFingerprint := presentationFingerprint{
					File: branch.Source.File, Line: branch.Source.Line, Scope: n.Scope,
					Kind: "Branch", Label: label, Header: branch.Code,
				}
				candidates = append(candidates, presentationCandidate{
					token: presentationToken(branchFingerprint),
					set:   func(key string) { branch.Key = key },
				})
			}
			for _, children := range n.Children {
				collect(children)
			}
		}
	}
	collect(nodes)

	counts := make(map[string]int, len(candidates))
	for _, candidate := range candidates {
		counts[candidate.token]++
	}
	for _, candidate := range candidates {
		if counts[candidate.token] == 1 {
			candidate.set(candidate.token)
		}
	}
}

func presentationToken(fingerprint presentationFingerprint) string {
	encoded, _ := json.Marshal(fingerprint)
	digest := sha256.Sum256(encoded)
	return "p" + hex.EncodeToString(digest[:8])
}
