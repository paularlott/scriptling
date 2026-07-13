import scriptling.similarity as sim

# cosine_similarity on known vectors
s = sim.cosine_similarity([1, 0, 0], [1, 0, 0])
assert abs(s - 1.0) < 0.0001, f"identical: {s}"

s = sim.cosine_similarity([1, 0], [0, 1])
assert abs(s - 0.0) < 0.0001, f"orthogonal: {s}"

s = sim.cosine_similarity([1, 1], [-1, -1])
assert abs(s - (-1.0)) < 0.0001, f"opposite: {s}"

# vectorize: same text → same vector, L2-normalised
v1 = sim.vectorize("hello world", dims=128)
v2 = sim.vectorize("hello world", dims=128)
assert len(v1) == 128, f"dims: {len(v1)}"
assert v1 == v2, "identical text should produce identical vectors"

# L2 normalised: magnitude ≈ 1
mag = sum(x * x for x in v1)
assert abs(mag - 1.0) < 0.001, f"not normalised: mag^2={mag}"

# Similar texts score higher than disjoint texts
v_rel = sim.vectorize("the quick brown fox", dims=256)
v_unrel = sim.vectorize("completely different words", dims=256)
v_query = sim.vectorize("the quick red fox", dims=256)
score_rel = sim.cosine_similarity(v_query, v_rel)
score_unrel = sim.cosine_similarity(v_query, v_unrel)
assert score_rel > score_unrel, f"related={score_rel} unrelated={score_unrel}"

# most_similar: rank vectors
query = sim.vectorize("hello", dims=64)
candidates = [
    sim.vectorize("hello world", dims=64),
    sim.vectorize("goodbye", dims=64),
    sim.vectorize("hello there", dims=64),
]
results = sim.most_similar(query, candidates, top_k=2)
assert len(results) == 2, f"expected 2 results, got {len(results)}"
assert results[0]["score"] >= results[1]["score"], "results not sorted"
top_indices = {r["index"] for r in results}
assert 0 in top_indices and 2 in top_indices, f"expected indices 0,2: {top_indices}"

True
