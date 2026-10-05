# Decision models: classification, yes/no probability and rubric scoring
# in one request (Ollama System One — clef-flash, clef, nimble, tev1).
#
# Unlike completion() a decision model never generates text: it scores the
# state against each named question and returns probabilities.
#
# Prerequisites:
# - Ollama v0.35.0+ running on http://localhost:11434
# - A decision model pulled: ollama pull clef-flash
#
# Run with: ../../bin/scriptling example.py [base_url] [model]

import scriptling.ai as ai
import sys

base_url = sys.argv[1] if len(sys.argv) >= 2 else "http://localhost:11434"
model = sys.argv[2] if len(sys.argv) >= 3 else "clef-flash"

client = ai.Client(base_url, provider=ai.OLLAMA)

state = """
Our checkout page has returned 500 errors for every customer since 9am.
Payment attempts time out, sales are fully stopped, and three enterprise
customers have opened urgent tickets.
"""

# One request, three question types judged against the same state.
result = client.decide(
    model,
    state,
    questions={
        # choice: pick from named options
        "label": {
            "type": "choice",
            "instructions": "Which team should this incident page?",
            "criteria": {
                "billing": "Payments, refunds and invoicing problems",
                "bug": "Software errors, crashes and regressions",
                "account": "Login and account access problems",
            },
        },
        # noul: probability of true (0-1)
        "urgent": {
            "type": "noul",
            "instructions": "Does this need immediate human attention?",
        },
        # score: position on an ordered rubric (probability-weighted)
        "severity": {
            "type": "score",
            "instructions": "How severe is the customer impact?",
            "criteria": [
                "cosmetic",
                "minor",
                "major",
                "critical",
                "outage",
            ],
        },
    },
)

label = result["answers"]["label"]
urgent = result["answers"]["urgent"]
severity = result["answers"]["severity"]
levels = len(severity["legend"])

print(f"Model: {result['model']}")
print()
print(f"Label:     {label['choice']}")
for name, probability in sorted(label["probabilities"].items(), key=lambda item: -item[1]):
    marker = " <- top" if name == label["choice"] else ""
    print(f"  {name:<10} {probability:.3f}{marker}")
print(f"  confidence {label['confidence']:.3f}")
print()
print(f"Urgent:    {urgent['noul']:.3f} probability of true")
print()
# score is the probability-weighted level; legend and probabilities are
# keyed by level index ("0" upward). Confidence is how concentrated the
# distribution is (0 = spread evenly, 1 = all mass on one level) — a low
# value with two dominant levels means the model is torn between them.
print(f"Severity:  {severity['score']:.2f} / {levels - 1} (probability-weighted level)")
for index, probability in sorted(severity["probabilities"].items(), key=lambda item: -item[1]):
    print(f"  {severity['legend'][index]:<10} {probability:.3f}")
print(f"  confidence {severity['confidence']:.3f} (0 = spread evenly, 1 = all on one level)")
print()
print(f"Usage: {result['usage']['input_tokens']} in / {result['usage']['output_tokens']} out")
