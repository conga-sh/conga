import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://docs.conga.sh',
  base: '/',
  integrations: [
    starlight({
      title: 'CONGA',
      description:
        'CONGA is a generic, batteries-included CLI framework for Go with link-time opt-in subsystems. Model the command line with Go structs: their fields and tags describe the flags, arguments, subcommands, configuration cascade, help screens, and shell completion.',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/conga-sh/conga',
        },
      ],
      sidebar: [
        {
          label: 'Tutorials',
          items: [
            'tutorials/quickstart',
          ],
        },
        {
          label: 'How-to Guides',
          items: [
            'how-to/declare-subcommands',
            'how-to/accept-positional-arguments',
            'how-to/manage-config-files',
            'how-to/bind-environment-variables',
            'how-to/load-env-files',
            'how-to/inspect-value-provenance',
            'how-to/hot-reload-config',
            'how-to/enable-shell-completion',
            'how-to/wire-custom-completion',
            'how-to/add-custom-flag-types',
            'how-to/test-a-command',
            'how-to/keep-root-reference',
            'how-to/lint-schemas',
            'how-to/examples-cookbook',
          ],
        },
        {
          label: 'Reference',
          items: [
            'reference/struct-tags',
            'reference/conga',
            'reference/config',
            'reference/predict',
            'reference/plugins',
            'reference/watch',
            'reference/decoders',
            'reference/builtins',
            'reference/errors',
            'reference/domain-types',
            'reference/binary-size-benchmark',
          ],
        },
        {
          label: 'Explanation',
          items: [
            'explanation/value-precedence',
            'explanation/config-cascade',
            'explanation/flags-and-args',
            'explanation/reading-values',
            'explanation/subcommands',
            'explanation/lifecycle',
            'explanation/execution',
            'explanation/error-reporting',
            'explanation/completion-architecture',
            'explanation/binary-size',
            'explanation/plugins-vs-predictors',
          ],
        },
      ],
    }),
  ],
});
