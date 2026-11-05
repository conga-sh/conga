# Writing Website Docs

Rules for pages under `website/src/content/docs/`. The site follows [Diátaxis](https://diataxis.fr/): `tutorials/` teach, `how-to/` direct a competent developer to a goal, `reference/` describes, and `explanation/` discusses. Each page serves one of these needs; move material that serves another need to the right page and link to it.

The developer reading a how-to wants a particular **behaviour of the CLI they are building**. The tag they write is the means, and what the person running the CLI types and sees is the result. Pages must show that result, not only the declaration.

## 1. End every how-to step with the result

After the code for a step, show what an end user of the built CLI does and gets: the command, then its output (help screen, error, `--why` table, or the program's own output).

- Generate output by running a program (preferably the matching `_examples/NN-*` program) and paste it verbatim into a `text no-copy` block. Never hand-write terminal output.
- When a step needs a variation the example does not have, run that variation and say in the prose what changed (for example, "Here `AdminAPI` was also given `env:"ENABLE_HTTP3"`").
- Use one running example for the whole page. Don't redefine the same struct differently in each section.

## 2. Add a "what users see" table for edge cases

When the same input can come from several sources, or fail several ways, give one table: rows are what the user does, columns are where it comes from (command line, environment, config file), and cells are the result (works, ignored, usage error exit 2, `ErrConfig` exit 1). See the "Know what users see when input is wrong" section of `how-to/feature-flags.md`.

## 3. Put surprising behaviour in the step, not in a note

State replacement semantics ("X replaces Y, it doesn't add to it"), things that don't apply ("`App.EnvPrefix` does not reach features"), and silent ignores in the step where the developer makes the choice. Use a short sentence plus a concrete example of what stops working. Keep `:::note` for real asides.

## 4. Keep the canonical rules in reference

Reference pages hold the full rule set for each tag family, including a "runtime behaviour" subsection with the canonical outcome table. How-tos repeat only what the reader needs to act, and link to reference for the rest. Explanation pages say *why*; they don't repeat rules or steps.

## 5. Prose

- Headings name the reader's goal in the imperative ("Gate a subcommand", "Give each feature its own environment variable"), not a noun ("Features section").
- Name the concept before using it, and keep distinct things visibly distinct in examples (a feature `http3` gating a flag `--quic`, not a flag `--http3`).
- State behaviour positively: say what the program does (appears, reads, reports) rather than what it doesn't. Keep negatives for real prohibitions, and put each one next to the alternative.
- When behaviour depends on a configuration choice, name the choice. Write "the variable named by the container's `env` tag", not "the environment", when the example's setup is only one of the options.
- Prefer everyday words to internal vocabulary: "source" over "tier", "the variable is ignored" over "the env surface is pruned".
- If a sentence could appear unchanged in another project's docs, it says nothing about CONGA. Rewrite or delete it.

## Verification

- Rerun every command shown on a page you changed and compare the output.
- `grep -rn "<slug>/#" website/src` after renaming a heading, and update every anchor.
- Build the site (`cd website && pnpm build`) and fix broken-link warnings.
