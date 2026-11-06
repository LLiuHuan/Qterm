import js from '@eslint/js'
import { withVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import eslintConfigPrettier from 'eslint-config-prettier'
import pluginVue from 'eslint-plugin-vue'
import globals from 'globals'

export default withVueTs(
  {
    ignores: ['bindings/**', 'coverage/**', 'dist/**', 'playwright-report/**', 'test-results/**', 'public/novnc/**'],
  },
  js.configs.recommended,
  pluginVue.configs['flat/recommended'],
  vueTsConfigs.recommended,
  {
    files: ['**/*.{ts,vue}'],
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node,
      },
    },
    rules: {
      'vue/attributes-order': 'off',
      'vue/max-attributes-per-line': 'off',
      'vue/multi-word-component-names': 'off',
    },
  },
  {
    files: ['src/components/ui/**/*.vue'],
    rules: {
      'vue/require-default-prop': 'off',
    },
  },
  {
    files: ['src/**/*.{ts,vue}'],

    ignores: ['src/components/common/**', 'src/components/ui/**', 'src/components/ai-elements/**', 'src/**/*.spec.ts'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: ['**/components/ui/**', '@/components/ui/**'],
              message:
                '基础控件必须从 components/common 下的 Qterm 通用控件引用（common / dialog-controls / form-controls / overlay-controls / control-primitives），不得直接依赖 components/ui。',
            },
          ],
        },
      ],
    },
  },
  {
    files: ['src/**/*.vue'],
    ignores: ['src/components/ui/**', 'src/**/*_bak.vue'],
    rules: {
      'vue/no-restricted-static-attribute': [
        'error',
        {
          key: 'type',
          value: 'number',
          element: 'QtermInput',
          message: '数字输入请使用 QtermNumberField',
        },
      ],
    },
  },
  eslintConfigPrettier,
)
