# Documentation Strategy Analyzer Prompt

A specialized LLM prompt that creates comprehensive documentation plans using the Diátaxis framework. This prompt transforms the LLM into a Documentation Strategy Analyst that systematically evaluates projects and generates structured documentation recommendations.

## Key Features

- Analyzes documentation needs across four Diátaxis categories:
  - Tutorials (Learning-oriented)
  - How-to Guides (Problem-oriented)
  - Reference (Information-oriented)
  - Explanation (Understanding-oriented)
- Generates YAML-formatted output with:
  - Input documentation requirements
  - Output documentation deliverables
  - Priority classifications
- Provides detailed rationale for each documentation need
- Considers both immediate and future documentation requirements

## Usage

Feed this prompt your project/system information to receive a comprehensive documentation strategy, including input requirements, output deliverables, and prioritized documentation needs.

## Output Format

Produces a structured YAML document detailing documentation needs, categorized by inputs, outputs, and priorities, with importance levels and timing recommendations.

## Model

This pattern was tested and tuned for GPT-4o