// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import sitemap from '@astrojs/sitemap';

// The site lives on GitHub Pages under /sumisura for now. A custom domain is
// deferred (see issue #142): moving to it means setting `site` to the domain,
// dropping `base`, and adding public/CNAME — nothing else, because every
// internal link is built from BASE_URL rather than hardcoded.
const SITE = 'https://gio-del.github.io';
const BASE = '/sumisura';

/**
 * Rewrites relative Markdown cross-links (`./lan-mode.md`) to real URLs under
 * the configured base. Docs therefore link by file, never by deployed path, so
 * changing `base` — or dropping it for a custom domain — needs no edits to any
 * content file. Astro does not do this rewriting on its own.
 */
function relativeDocLinks() {
  const rewrite = (node) => {
    if (node.type === 'link' && typeof node.url === 'string') {
      const match = /^\.\/?([\w-]+)\.md(#.*)?$/.exec(node.url);
      if (match) node.url = `${BASE}/${match[1]}/${match[2] ?? ''}`;
    }
    for (const child of node.children ?? []) rewrite(child);
  };
  return (tree) => rewrite(tree);
}

export default defineConfig({
  site: SITE,
  base: BASE,
  markdown: { remarkPlugins: [relativeDocLinks] },
  trailingSlash: 'ignore',
  integrations: [
    sitemap(),
    starlight({
      title: 'Sumisura',
      description:
        'Self-hosted, AI-tailored CVs and cover letters that stay grounded in your real career history.',
      logo: {
        light: './src/assets/logo-lockup.svg',
        dark: './src/assets/logo-lockup-dark.svg',
        replacesTitle: true,
      },
      favicon: '/favicon.svg',
      customCss: [
        // Self-hosted, like the app's fonts: no third-party requests from the
        // site either, so "your data stays yours" holds on the page that says it.
        '@fontsource/inter/400.css',
        '@fontsource/inter/600.css',
        '@fontsource/instrument-serif/400.css',
        './src/styles/brand.css',
      ],
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/gio-del/sumisura' },
      ],
      editLink: {
        baseUrl: 'https://github.com/gio-del/sumisura/edit/main/site/',
      },
      sidebar: [
        {
          label: 'Start here',
          items: [
            { label: 'Quickstart', slug: 'quickstart' },
            { label: 'How it works', slug: 'concepts' },
          ],
        },
        {
          label: 'Using Sumisura',
          items: [
            { label: 'Writing your Master Data', slug: 'master-data' },
            { label: 'Tailoring a CV', slug: 'tailoring' },
            { label: 'Tracking applications', slug: 'tracking' },
            { label: 'Browser extension', slug: 'extension' },
          ],
        },
        {
          label: 'Running it',
          items: [
            { label: 'Configuration', slug: 'configuration' },
            { label: 'LAN-reachable mode', slug: 'lan-mode' },
            { label: 'Remote access', slug: 'remote-access' },
            { label: 'Capture from your phone', slug: 'phone-capture' },
            { label: 'Upgrading', slug: 'upgrading' },
            { label: 'Troubleshooting', slug: 'troubleshooting' },
            { label: 'API reference', slug: 'api' },
          ],
        },
        {
          label: 'Contributing',
          items: [{ label: 'Contributing', slug: 'contributing' }],
        },
      ],
    }),
  ],
});
