import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  // Generated Wails bindings and build output are not hand-written code.
  { ignores: ['dist', 'bindings'] },
  {
    files: ['**/*.{ts,tsx}'],
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
    },
    plugins: {
      'react-hooks': reactHooks,
    },
    rules: {
      // Only the classic hook rules; the plugin's React Compiler rules are not adopted yet.
      'react-hooks/rules-of-hooks': 'error',
      'react-hooks/exhaustive-deps': 'warn',
      // docs/dev/00.rules.md: explicit types, no `any` unless unavoidable.
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
    },
  },
  {
    // vite.config.ts is outside the tsc project; its @ts-ignore comments only quiet editor checks,
    // so @ts-expect-error would be flagged as unused wherever the editor reports nothing.
    files: ['vite.config.ts'],
    rules: { '@typescript-eslint/ban-ts-comment': 'off' },
  },
)
