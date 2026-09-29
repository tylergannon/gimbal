# Mathematical analysis of bounded claim screening

This note separates a theorem about an **exact** pair-conflict oracle from an
operating policy for Jev. Let `V` contain `N` original claim records. Let `E`
be an arbitrary symmetric, irreflexive relation on `V`; the requested output is
the set of endpoints of edges in `E`, not necessarily the edge list. `W(S)`
answers whether `E` has an edge wholly inside `S`. `X(A,B)` answers whether an
edge joins disjoint `A` and `B`. Both return exact Booleans, and a request
contains at most `K >= 2` **full original claims**. This model assumes that
scope and applicability have already been resolved well enough for `E` to be
defined. It does not cover a contradiction that exists only among three or
more jointly inconsistent claims.

## Coverage and lower bound

Partition `V` into batches `B_1,...,B_m` of at most `b` claims, where
`2b <= K` and `m = ceil(N/b)`. Every unordered pair lies either within one
batch or across one pair of batches. Therefore `W(B_i)` for each nontrivial
batch and `X(B_i,B_j)` for every `i < j` cover all pairs. Internal batch checks
alone do not cover cross-batch conflicts. If all answers are exact negatives,
the schedule takes at most `m + binom(m,2) = m(m+1)/2` requests; omit singleton
internal queries. For `N=1000,b=10`, the bound is 5,050 requests versus
499,500 single-pair questions. These are HTTP request counts, not numbers of
semantic pair judgments in a multi-question request.

The quadratic exponent is unavoidable for arbitrary `E` and bounded `K` in
this oracle model, even though the output consists only of marked vertices.
Run any correct adaptive algorithm on the all-clean graph. If some pair
`{u,v}` never appears together in a query, replace the clean graph by the graph
whose sole edge is `{u,v}`. Every answer in the clean transcript is unchanged,
so the algorithm cannot know to mark `u` and `v`. Thus the queried vertex sets
must cover all `binom(N,2)` pairs. Each query contains at most `binom(K,2)`
pairs, giving at least `ceil(binom(N,2)/binom(K,2))` queries. This is
`Omega(N^2/K^2)` when `N` is large relative to `K`; with fixed `K`, it is
quadratic. If an oracle may inspect all `N` claims at once, if the graph has
known structure, or if a deterministic rule can infer conflicts without
co-presenting claims, this lower bound does not apply.

Overlapping or repeatedly recombined batches do not evade this argument. On a
clean transcript, their queried claim sets form a pair-covering design: every
possible pair must co-occur in at least one request, irrespective of how the
sets were chosen or whether earlier batches overlapped. Testing only each
batch internally misses conflicts whose endpoints never meet. The within-batch
and every-cross-batch schedule above is a simple explicit pair cover; cleverer
overlaps can change constants but cannot beat the stated lower bound for fixed
`K`. Random reshuffling has the same distinction. For `2 <= K < N`, if each
of `r` independent rounds uniformly shuffles `N` claims and partitions the
shuffle into groups of at most `K`, a particular
hidden pair meets in a round with probability at most `(K-1)/(N-1)` (exactly
that value for equal-sized groups of size `K` when `K` divides `N`). Its chance
of remaining unseen is then at least `(1-(K-1)/(N-1))^r`; a finite `r` leaves
a nonzero miss chance. Randomization can give a stated probabilistic detection
target under explicit assumptions, but cannot certify complete coverage unless
the realized requests are checked to cover every pair. Even that coverage only
guarantees scheduling; fallible model answers may still miss a co-present pair.

## Localization and the smaller vertex-marking search

An exact positive `W(S)` yields one witness edge in `O(log |S|)` queries:
halve `S` into `L,R`, test `W(L)` and then `W(R)` if needed; if both are
negative, the edge crosses and `X(L,R)` is known positive. To localize a
positive `X(A,B)`, halve the larger side, test one half against the other side,
and retain the positive half (the other is positive if the tested half is
negative). Each step cuts the larger side; in at most `O(log K)` steps both
sides are singletons. This finds **one** edge, not every edge. Exhaustively
descending every positive region to pairs is correct under exact answers, but
can spend `Theta(K^2)` queries within one dense region.

Because the output is vertices, exact-oracle search can do less in a positive
region. Maintain `M`, the vertices with retained **confirmed pair witnesses**,
and `U = V \ M`. Within a batch, only `W(U_i)` and `X(U_i,M_i)` can mark a new
vertex. Across batches `i,j`, only these three rectangles can do so:
`X(U_i,U_j)`, `X(U_i,M_j)`, and `X(M_i,U_j)`. Edges within `M_i` or between
`M_i` and `M_j` cannot change the requested output. Test relevant nonempty
regions; on a positive answer locate one edge as above, retain that concrete
witness, move its previously unmarked endpoint(s) into `M`, and test the
updated relevant regions again. Stop a batch or batch pair when all its
relevant regions answer negative. Never remove a marked vertex from future
cross tests against unmarked vertices.

Invariant: every member of `M` has a retained exact witness, and an exact
negative region proves there is no edge touching its currently unmarked
members in that region. The algorithm terminates because every positive
iteration marks at least one new vertex. A terminal batch/batch-pair has no
edge with an unmarked endpoint. These regions partition all possible edges,
so after processing all batches and batch pairs, `M` is exactly the endpoint
set of `E`. A cleared region need not be revisited in a fixed snapshot: `U`
only shrinks, and an exact negative is hereditary to subsets. An edit to a
claim, its scope, or the oracle policy invalidates that reasoning and requires
the affected regions to be checked again.

Each successful witness adds at least one previously unmarked vertex, so there
are at most `N` such iterations **globally**, not per block pair. A batch or
batch pair needs at most three relevant-screen queries per iteration plus a
constant number of terminal screens; each witness costs `O(log K)` queries to
localize. With `m = ceil(N/b)`, the total is
`O(m^2 + N log K)` exact-oracle requests. For a single positive region of at
most `K` claims, the bound is `O(K log K)` after its initial screen. This is an
output-sensitive upper bound on request count; all-clean inputs still incur
the pair-cover lower bound. The algorithm need not emit all edges, but each
marked vertex must retain at least one actual pair witness.

## Tokens and structured cases

In the clean batch schedule, each claim appears in its own within-batch
request and in every cross-batch request involving that batch. Thus the number
of transmitted original-claim occurrences is at most `N*m`, or
`O(N^2/b + N)` for balanced batches. With bounded claim length, this is also
the claim-text token order. The request count is `O(N^2/b^2 + N/b)`.
Repeated instructions, source passages, scoped context, and follow-up
questions require separate accounting. The vertex-marking search adds at most
`O(N K log K)` claim occurrences by the crude bound of `K` claims per witness
query. A hypothetical whole-index clean screen uses one request but still
reads `Theta(N)` claim text.

There is a genuine simpler special case: if normalized records assert an exact
value for the **same known single-valued property under identical scope**,
group by that key. A key with more than one distinct value marks every record
in the key's group. Hashing is expected `O(N)` and sorting `O(N log N)`,
excluding semantic normalization. This rule does not extend to arbitrary
natural-language topic buckets, ranges, aliases, unknown time scopes,
implications, or universal/specific claims. For example, `x in {1,2}`,
`x != 1`, and `x != 2` are pairwise compatible yet jointly inconsistent;
pairwise `E` intentionally misses that kind of inconsistency.

## What Jev can support

Jev's `no-conflict` and `conflict` answers are fallible semantic judgments, not
the exact `W` and `X` of these proofs. A finite probe, including a synthetic
batch-size trial, cannot establish zero false negatives. An uncertain answer
or a possible-conflict marker is not a confirmed witness and **must not move a
vertex from `U` to `M` for pruning**. A false negative would invalidate both
the block-cover guarantee and the hereditary-clearance invariant. Without a
validated policy for accepting negative screens, the conservative procedure is
to ask explicit pair questions (which may be packed into shared-state
requests), retain unresolved answers, and mark confirmed and possible disputes
distinctly. This simpler baseline avoids recursive group machinery while
group-negative pruning remains unproven. Even explicit pair judgments cannot
prove factual truth or perfect contradiction recall from a fallible model;
they establish complete **scheduling** and an inspectable set of judgments.
