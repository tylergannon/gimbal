# Core premises and claims for independent mathematical review

This is a list to prove, qualify, or refute, not an established theorem.
The user asks for every index claim to be checked against its cited sources and
contradictory claims to be marked. They propose screening groups of roughly
10–20 claims for coherence, then locating contradictory pairs in suspect groups.
They ask whether the work can be O(n log n) or better and accept quadratic work
if necessary. Full context: [request.md](request.md).

## Model and boundaries

Let V be the N distinct claim records. A symmetric, irreflexive relation E
represents pairwise contradiction after retaining subject, time/version,
conditions, and measurement definitions. The requested marks identify vertices
incident to E; outputting every edge is not required.

For mathematical analysis only, assume an exact oracle W(S) that reports
whether the induced subgraph on S contains any edge, and an exact oracle
X(A,B) that reports whether an edge crosses disjoint sets A and B. Each oracle
request can contain at most K original claims; original claim length is bounded
only where a token-complexity claim explicitly assumes that. Neither oracle
answers contradictions involving three or more statements but no incompatible
pair. W and X are semantic models of requests, not claims of Jev accuracy.

The real Jev oracle is fallible, may answer uncertain, and has bounded token
context. A calibrated negative answer is empirical evidence, never a formal
proof that the input set is conflict-free. Exactness of scheduling is distinct
from exactness of model decisions and claim extraction.

## Candidate claims

1. **Coverage by blocks.** Partition V into disjoint base batches B1...Bm,
   each of size at most b, with 2b <= K. Checking W(Bi) for each i and
   X(Bi,Bj) for each i<j covers every possible unordered pair once at the
   top level. Internal coherence alone cannot discharge cross-batch pairs.

2. **Recursive localization.** For positive/uncertain W(S), split S into L,R
   and inspect W(L), W(R), and X(L,R). For positive/uncertain X(A,B), split
   the larger side and inspect both resulting cross regions. Negative exact
   oracle results prune their region. At singleton-pair leaves, inspect that
   pair. This finds all contradictory pairs if every unresolved region is
   explored, which also suffices to mark all affected claims. Removing the
   endpoints of a discovered edge can miss other affected claims.

3. **Clean-input request count.** If all top-level queries return accepted
   negatives, Q=m+m(m-1)/2 with m=ceil(N/b), except trivial singleton internal
   queries can be omitted. N=1000,b=10 gives 5050 top-level requests versus
   499500 individual unordered pairs. This comparison is against individual
   questions/requests; shared-state explicit pair-question batching is another
   baseline, not the same cost model.

4. **Bounded-oracle lower bound.** On an all-clean input, any algorithm that
   must correctly mark every contradictory vertex for an arbitrary relation E,
   and can learn E only through exact queries containing at most K vertices,
   needs at least ceil(binomial(N,2)/binomial(K,2)) such queries in the worst
   case. Otherwise an unqueried pair could hide the sole edge. This claim is
   restricted to this oracle model and requires K>=2. The small output size
   does not defeat the hidden-single-edge argument.

5. **Local witness versus global work.** Once a bounded group is known to
   contain an edge, a first witness can be located with O(log K) adaptive
   oracle queries using the internal/cross distinction. That does not imply
   O(N log N) whole-index coverage. Exhaustively descending all batch searches
   is O(N²) oracle judgments for bounded-size starting batches, including the
   constant-factor internal nodes, assuming constant-cost leaf judgments.

6. **Token volume.** In the clean top-level schedule, with bounded-length
   claims and full original claims included in every request, claim-text
   volume is O(N²/b + N) and request count is O(N²/b² + N/b). Source payloads,
   follow-up questions, and batching multiple questions change constants or
   require their own accounting. A hypothetical whole-index oracle can clear
   a clean index in one call but still reads O(N) claim text.

7. **Structured shortcuts.** For already normalized statements assigning
   exact values to a genuinely single-valued property under identical known
   scope, hashing/sorting can identify incompatible value groups and mark all
   affected claims in expected O(N) / O(N log N), without emitting all edges.
   This is not valid for arbitrary natural-language bucket labels, unknown
   scopes, ranges, implications, aliases, or universal/specific relationships.

8. **Practical negative policy.** No finite synthetic probe establishes that
   Jev will never miss a conflict. Before a domain-specific pruning policy is
   established, group questions can prioritize work but cannot justify
   skipping pair checks under the proposed conservative starting policy.
   Uncertain or conflicting oracle answers leave a visible unresolved region.

The full current proposal is [proposal.md](proposal.md). Review these claims
independently, including the assumptions and whether this oracle model answers
the user's actual question. Prefer a corrected simpler algorithm if warranted;
do not defend the candidate claims merely because they appear here.
