import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

function pluginNoCopy() {
  return {
    name: 'NoCopy',
    hooks: {
      postprocessRenderedBlock: ({ codeBlock, renderData }) => {
        const disableCopy =
          Boolean(codeBlock.metaOptions.getBoolean('no-copy')) ||
          codeBlock.metaOptions.getBoolean('copy') === false ||
          codeBlock.meta?.split(/\s+/).includes('no-copy') ||
          codeBlock.language === 'output';

        if (disableCopy && renderData.blockAst) {
          renderData.blockAst.children = renderData.blockAst.children.filter(
            (child) =>
              !(
                child.type === 'element' &&
                (Array.isArray(child.properties?.className)
                  ? child.properties.className.includes('copy')
                  : child.properties?.className === 'copy')
              )
          );
        }
      },
    },
  };
}

export default defineConfig({
  site: 'https://docs.conga.sh',
  base: '/',
  redirects: {
    '/how-to/supply-config-from-code': '/how-to/load-domain-files',
  },
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
      customCss: ['./src/styles/custom.css'],
      expressiveCode: {
        plugins: [pluginNoCopy()],
      },
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
            { label: 'Declare subcommands', slug: 'how-to/declare-subcommands' },
            { label: 'Accept positional arguments', slug: 'how-to/accept-positional-arguments' },
            { label: 'Customize help screens', slug: 'how-to/customize-help-screens' },
            { label: 'Manage config files', slug: 'how-to/manage-config-files' },
            { label: 'Load domain documents', slug: 'how-to/load-domain-files' },
            { label: 'Bind environment variables', slug: 'how-to/bind-environment-variables' },
            { label: 'Load .env files', slug: 'how-to/load-env-files' },
            { label: 'Gate features', slug: 'how-to/feature-flags' },
            { label: 'Inspect value provenance', slug: 'how-to/inspect-value-provenance' },
            { label: 'Enable shell completion', slug: 'how-to/enable-shell-completion' },
            { label: 'Wire custom completion', slug: 'how-to/wire-custom-completion' },
            { label: 'Add custom flag types', slug: 'how-to/add-custom-flag-types' },
            { label: 'Test commands', slug: 'how-to/test-a-command' },
            { label: 'Retain root struct', slug: 'how-to/keep-root-reference' },
            { label: 'Lint schemas', slug: 'how-to/lint-schemas' },
            { label: 'Run examples', slug: 'how-to/examples-cookbook' },
          ],
        },
        {
          label: 'Reference',
          items: [
            'reference/struct-tags',
            'reference/conga',
            'reference/predict',
            'reference/decoders',
            'reference/dotenv',
            'reference/builtins',
            'reference/errors',
            'reference/conga-lint',
            'reference/domain-types',
          ],
        },
        {
          label: 'Explanation',
          items: [
            'explanation/value-precedence',
            'explanation/config-cascade',
            'explanation/flags-and-args',
            'explanation/subcommands',
            'explanation/feature-toggles',
            'explanation/lifecycle',
            'explanation/error-reporting',
            'explanation/domain-kits',
          ],
        },
      ],
    }),
  ],
});
