import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import codeblocks from 'starlight-codeblocks';
import starlightLinksValidator from 'starlight-links-validator';

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
    '/how-to/supply-config-from-code': '/how-to/load-domain-documents',
    '/how-to/examples-cookbook': '/reference/examples',
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
      editLink: {
        baseUrl: 'https://github.com/conga-sh/conga/edit/main/website/',
      },
      customCss: ['./src/styles/custom.css'],
      plugins: [
        codeblocks({
          fileIcons: {
            files: { '.elv': 'file-type-shell', '.env': 'file-type-dotenv' },
          },
        }),
        starlightLinksValidator(),
      ],
      expressiveCode: {
        plugins: [pluginNoCopy()],
      },
      sidebar: [
        {
          label: 'Tutorials',
          items: [
            'tutorials/quickstart',
            'tutorials/multi-command-cli',
            'tutorials/configure-a-cli',
            'tutorials/test-a-cli',
            'tutorials/complete-a-cli',
          ],
        },
        {
          label: 'How-to Guides',
          items: [
            { label: 'Declare flags', slug: 'how-to/declare-flags' },
            { label: 'Validate input', slug: 'how-to/validate-input' },
            { label: 'Declare subcommands', slug: 'how-to/declare-subcommands' },
            { label: 'Accept positional arguments', slug: 'how-to/accept-positional-arguments' },
            { label: 'Read files and standard input', slug: 'how-to/read-files-and-stdin' },
            { label: 'Use network, system, and timezone types', slug: 'how-to/use-domain-packages' },
            { label: 'Customize help screens', slug: 'how-to/customize-help-screens' },
            { label: 'Manage config files', slug: 'how-to/manage-config-files' },
            { label: 'Load domain documents', slug: 'how-to/load-domain-documents' },
            { label: 'Bind environment variables', slug: 'how-to/bind-environment-variables' },
            { label: 'Load .env files', slug: 'how-to/load-env-files' },
            { label: 'Gate features', slug: 'how-to/gate-behind-features' },
            { label: 'Inspect value provenance', slug: 'how-to/inspect-value-provenance' },
            { label: 'Enable shell completion', slug: 'how-to/enable-shell-completion' },
            { label: 'Complete from tags and types', slug: 'how-to/complete-from-tags-and-types' },
            { label: 'Compute and shape completion candidates', slug: 'how-to/compute-completion-candidates' },
            { label: 'Add custom flag types', slug: 'how-to/add-custom-flag-types' },
            { label: 'Test commands', slug: 'how-to/test-a-command' },
            { label: 'Read values after Run', slug: 'how-to/read-values-after-run' },
            { label: 'Lint schemas', slug: 'how-to/lint-schemas' },
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
            'reference/examples',
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
            'explanation/testing',
            'explanation/domain-packages',
          ],
        },
      ],
    }),
  ],
});
