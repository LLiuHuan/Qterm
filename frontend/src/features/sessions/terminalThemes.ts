import { getPreference, PREFERENCE_KEYS, readCachedPreference, setPreference } from '../../services/preferences'
import { translate } from '@/locales'

export interface TerminalTheme {
  name: string
  type: 'light' | 'dark'
  background: string
  foreground: string
  cursor: string
  cursorAccent: string
  selectionBackground: string
  black: string
  red: string
  green: string
  yellow: string
  blue: string
  magenta: string
  cyan: string
  white: string
  brightBlack: string
  brightRed: string
  brightGreen: string
  brightYellow: string
  brightBlue: string
  brightMagenta: string
  brightCyan: string
  brightWhite: string
}

export type TerminalThemeId = string
export const CUSTOM_TERMINAL_THEMES_KEY = 'qterm.terminal-themes.v1'

const base = (theme: Omit<TerminalTheme, 'name'> & { name: string }): TerminalTheme => theme

const localizedBuiltinTheme = (theme: TerminalTheme, key: string): TerminalTheme => ({
  ...theme,
  get name() {
    return translate(key)
  },
})

const paletteTheme = (
  name: string,
  type: 'light' | 'dark',
  background: string,
  foreground: string,
  colors: [string, string, string, string, string, string, string, string],
  brightColors: [string, string, string, string, string, string, string, string],
  selectionBackground = type === 'light' ? '#d6d3d1' : '#3f3f46',
): TerminalTheme => {
  const [black, red, green, yellow, blue, magenta, cyan, white] = colors
  const [brightBlack, brightRed, brightGreen, brightYellow, brightBlue, brightMagenta, brightCyan, brightWhite] =
    brightColors
  return base({
    name,
    type,
    background,
    foreground,
    cursor: foreground,
    cursorAccent: background,
    selectionBackground,
    black,
    red,
    green,
    yellow,
    blue,
    magenta,
    cyan,
    white,
    brightBlack,
    brightRed,
    brightGreen,
    brightYellow,
    brightBlue,
    brightMagenta,
    brightCyan,
    brightWhite,
  })
}

const ADDITIONAL_TERMINAL_THEMES: Record<string, TerminalTheme> = {
  'termius-light': paletteTheme(
    'Termius Light',
    'light',
    '#d5dde0',
    '#32364a',
    ['#141729', '#f24e50', '#198c51', '#ab6918', '#004878', '#8f3c91', '#2091f6', '#eeeeee'],
    ['#3e4257', '#ff7375', '#21b568', '#fdc47d', '#1d6da2', '#ff7dc5', '#44a7ff', '#ffffff'],
    '#32364a80',
  ),
  'termius-dark': paletteTheme(
    'Termius Dark',
    'dark',
    '#141729',
    '#21b568',
    ['#343851', '#f24e50', '#008463', '#eca855', '#08639f', '#c13282', '#2091f6', '#e2e3e8'],
    ['#8d91a5', '#ff7375', '#3ed7be', '#fdc47d', '#6ba0c3', '#ff7dc5', '#44a7ff', '#ffffff'],
    '#21b56880',
  ),
  'solarized-dark': paletteTheme(
    'Solarized Dark',
    'dark',
    '#002b36',
    '#839496',
    ['#073642', '#dc322f', '#859900', '#b58900', '#268bd2', '#d33682', '#2aa198', '#eee8d5'],
    ['#002b36', '#cb4b16', '#586e75', '#657b83', '#839496', '#6c71c4', '#93a1a1', '#fdf6e3'],
    '#073642',
  ),
  'solarized-light': paletteTheme(
    'Solarized Light',
    'light',
    '#fdf6e3',
    '#657b83',
    ['#073642', '#dc322f', '#859900', '#b58900', '#268bd2', '#d33682', '#2aa198', '#eee8d5'],
    ['#002b36', '#cb4b16', '#586e75', '#657b83', '#839496', '#6c71c4', '#93a1a1', '#fdf6e3'],
    '#eee8d5',
  ),
  'ayu-dark': paletteTheme(
    'Ayu Dark',
    'dark',
    '#0f1419',
    '#e6e1cf',
    ['#000000', '#ff3333', '#86b300', '#ffb454', '#36a3d9', '#f07178', '#95e1d3', '#ffffff'],
    ['#323232', '#ff6565', '#b8e536', '#ffc66d', '#55b4d4', '#ff8888', '#95e1d3', '#ffffff'],
    '#1f2430',
  ),
  'ayu-light': paletteTheme(
    'Ayu Light',
    'light',
    '#fafafa',
    '#5c6166',
    ['#000000', '#f07171', '#86b300', '#f2ae49', '#399ee6', '#a37acc', '#4cbf99', '#ffffff'],
    ['#323232', '#f07171', '#86b300', '#f2ae49', '#399ee6', '#a37acc', '#4cbf99', '#ffffff'],
    '#d6e4f0',
  ),
  cobalt2: paletteTheme(
    'Cobalt2',
    'dark',
    '#132738',
    '#ffffff',
    ['#000000', '#ff5d38', '#3ad900', '#ffb402', '#0088ff', '#bc3fbc', '#00d0d0', '#d0d0d0'],
    ['#808080', '#ff8070', '#66ff00', '#ffeb3b', '#5ba3ff', '#ff4eff', '#5ce8e8', '#ffffff'],
    '#1e3c41',
  ),
  cyberpunk: paletteTheme(
    'Cyberpunk',
    'dark',
    '#0d0221',
    '#ff006e',
    ['#0d0221', '#ff006e', '#8338ec', '#ffbe0b', '#3a86ff', '#ff006e', '#06ffa5', '#ffffff'],
    ['#6c5b7b', '#ff006e', '#8338ec', '#ffbe0b', '#3a86ff', '#ff006e', '#06ffa5', '#ffffff'],
    '#3a0ca3',
  ),
  'hacker-green': paletteTheme(
    'Hacker Green',
    'dark',
    '#0d0208',
    '#00ff41',
    ['#000000', '#ff0000', '#00ff41', '#008f11', '#005f00', '#00ff41', '#00ff41', '#00ff41'],
    ['#001100', '#ff0000', '#00ff41', '#00ff41', '#00ff41', '#00ff41', '#00ff41', '#ccffcc'],
    '#003b00',
  ),
  dracula: paletteTheme(
    'Dracula',
    'dark',
    '#282a36',
    '#f8f8f2',
    ['#21222c', '#ff5555', '#50fa7b', '#f1fa8c', '#bd93f9', '#ff79c6', '#8be9fd', '#f8f8f2'],
    ['#6272a4', '#ff6e6e', '#69ff94', '#ffffa5', '#d6acff', '#ff92df', '#a4ffff', '#ffffff'],
    '#44475a',
  ),
  nord: paletteTheme(
    'Nord',
    'dark',
    '#2e3440',
    '#d8dee9',
    ['#3b4252', '#bf616a', '#a3be8c', '#ebcb8b', '#81a1c1', '#b48ead', '#88c0d0', '#e5e9f0'],
    ['#4c566a', '#bf616a', '#a3be8c', '#ebcb8b', '#81a1c1', '#b48ead', '#8fbcbb', '#eceff4'],
    '#434c5e',
  ),
  'gruvbox-dark': paletteTheme(
    'Gruvbox Dark',
    'dark',
    '#282828',
    '#ebdbb2',
    ['#282828', '#cc241d', '#98971a', '#d79921', '#458588', '#b16286', '#689d6a', '#a89984'],
    ['#928374', '#fb4934', '#b8bb26', '#fabd2f', '#83a598', '#d3869b', '#8ec07c', '#ebdbb2'],
    '#504945',
  ),
  'gruvbox-light': paletteTheme(
    'Gruvbox Light',
    'light',
    '#fbf1c7',
    '#3c3836',
    ['#fbf1c7', '#cc241d', '#98971a', '#d79921', '#458588', '#b16286', '#689d6a', '#7c6f64'],
    ['#928374', '#9d0006', '#79740e', '#b57614', '#076678', '#8f3f71', '#427b58', '#3c3836'],
    '#ebdbb2',
  ),
  'one-dark': paletteTheme(
    'One Dark',
    'dark',
    '#282c34',
    '#abb2bf',
    ['#1e2127', '#e06c75', '#98c379', '#e5c07b', '#61afef', '#c678dd', '#56b6c2', '#abb2bf'],
    ['#5c6370', '#e06c75', '#98c379', '#e5c07b', '#61afef', '#c678dd', '#56b6c2', '#ffffff'],
    '#3e4451',
  ),
  'one-light': paletteTheme(
    'One Light',
    'light',
    '#fafafa',
    '#383a42',
    ['#383a42', '#e45649', '#50a14f', '#c18401', '#4078f2', '#a626a4', '#0184bc', '#a0a1a7'],
    ['#696c77', '#e45649', '#50a14f', '#c18401', '#4078f2', '#a626a4', '#0184bc', '#ffffff'],
    '#e5e5e6',
  ),
  'tokyo-night': paletteTheme(
    'Tokyo Night',
    'dark',
    '#1a1b26',
    '#a9b1d6',
    ['#15161e', '#f7768e', '#41a6b5', '#e0af68', '#7aa2f7', '#bb9af7', '#7dcfff', '#787c99'],
    ['#414868', '#f7768e', '#73daca', '#e0af68', '#7aa2f7', '#bb9af7', '#7dcfff', '#acb0d0'],
    '#283457',
  ),
  'tokyo-night-day': paletteTheme(
    'Tokyo Night Day',
    'light',
    '#e1e2e7',
    '#3760bf',
    ['#0f0f14', '#8c4351', '#33635c', '#8f5e15', '#34548a', '#5a4a78', '#0f4b6e', '#828594'],
    ['#828594', '#8c4351', '#33635c', '#8f5e15', '#34548a', '#5a4a78', '#0f4b6e', '#828594'],
    '#c4c8da',
  ),
  'catppuccin-mocha': paletteTheme(
    'Catppuccin Mocha',
    'dark',
    '#1e1e2e',
    '#cdd6f4',
    ['#45475a', '#f38ba8', '#a6e3a1', '#f9e2af', '#89b4fa', '#f5c2e7', '#94e2d5', '#bac2de'],
    ['#585b70', '#f38ba8', '#a6e3a1', '#f9e2af', '#89b4fa', '#f5c2e7', '#94e2d5', '#a6adc8'],
    '#313244',
  ),
  'catppuccin-latte': paletteTheme(
    'Catppuccin Latte',
    'light',
    '#eff1f5',
    '#4c4f69',
    ['#5c5f77', '#d20f39', '#40a02b', '#df8e1d', '#1e66f5', '#8839ef', '#179299', '#acb0be'],
    ['#6c6f85', '#d20f39', '#40a02b', '#df8e1d', '#1e66f5', '#8839ef', '#179299', '#bcc0cc'],
    '#ccd0da',
  ),
  monokai: paletteTheme(
    'Monokai',
    'dark',
    '#272822',
    '#f8f8f2',
    ['#272822', '#f92672', '#a6e22e', '#f4bf75', '#66d9ef', '#ae81ff', '#a6e22e', '#f8f8f2'],
    ['#75715e', '#f92672', '#a6e22e', '#f4bf75', '#66d9ef', '#ae81ff', '#a6e22e', '#f9f8f5'],
    '#49483e',
  ),
  'material-ocean': paletteTheme(
    'Material Ocean',
    'dark',
    '#0f111a',
    '#8f93a2',
    ['#000000', '#f07178', '#c3e88d', '#ffcb6b', '#82aaff', '#c792ea', '#89ddff', '#ffffff'],
    ['#546e7a', '#f07178', '#c3e88d', '#ffcb6b', '#82aaff', '#c792ea', '#89ddff', '#ffffff'],
    '#292d3e',
  ),
  'tomorrow-night': paletteTheme(
    'Tomorrow Night',
    'dark',
    '#1d1f21',
    '#c5c8c6',
    ['#1d1f21', '#cc6666', '#b5bd68', '#f0c674', '#81a2be', '#b294bb', '#8abeb7', '#c5c8c6'],
    ['#969896', '#cc6666', '#b5bd68', '#f0c674', '#81a2be', '#b294bb', '#8abeb7', '#ffffff'],
    '#373b41',
  ),
  tomorrow: paletteTheme(
    'Tomorrow',
    'light',
    '#ffffff',
    '#4d4d4c',
    ['#000000', '#c82829', '#718c00', '#eab700', '#4271ae', '#89569c', '#3e999f', '#ffffff'],
    ['#8e908c', '#c82829', '#718c00', '#eab700', '#4271ae', '#89569c', '#3e999f', '#ffffff'],
    '#d6d6d6',
  ),
  'night-owl': paletteTheme(
    'Night Owl',
    'dark',
    '#011627',
    '#d6deeb',
    ['#011627', '#ef5350', '#addb67', '#ecc48d', '#82aaff', '#c792ea', '#7fdbca', '#ffffff'],
    ['#575656', '#ef5350', '#addb67', '#ecc48d', '#82aaff', '#c792ea', '#7fdbca', '#ffffff'],
    '#1d3b53',
  ),
  'shades-of-purple': paletteTheme(
    'Shades of Purple',
    'dark',
    '#2d2b55',
    '#ffffff',
    ['#000000', '#ff628c', '#a5ff90', '#ffcc99', '#80cbc4', '#c792ea', '#addb67', '#ffffff'],
    ['#808080', '#ff628c', '#a5ff90', '#ffcc99', '#80cbc4', '#c792ea', '#addb67', '#ffffff'],
    '#494674',
  ),
  'everforest-dark': paletteTheme(
    'Everforest Dark',
    'dark',
    '#2d353b',
    '#d3c6aa',
    ['#4b565c', '#e67e80', '#a7c080', '#dbbc7f', '#7fbbb3', '#d699b6', '#83c092', '#d3c6aa'],
    ['#859289', '#e67e80', '#a7c080', '#dbbc7f', '#7fbbb3', '#d699b6', '#83c092', '#f2efdf'],
    '#475258',
  ),
  'everforest-light': paletteTheme(
    'Everforest Light',
    'light',
    '#fdf6e3',
    '#5c6a72',
    ['#5c6a72', '#f85552', '#8da101', '#dfa000', '#3a94c5', '#df69ba', '#35a77c', '#f2efdf'],
    ['#939f91', '#f85552', '#8da101', '#dfa000', '#3a94c5', '#df69ba', '#35a77c', '#ffffff'],
    '#e8e5d0',
  ),
  'kanagawa-wave': paletteTheme(
    'Kanagawa Wave',
    'dark',
    '#1f1f28',
    '#dcd7ba',
    ['#16161d', '#c34043', '#76946a', '#c0a36e', '#7e9cd8', '#957fb8', '#6a9589', '#c8c093'],
    ['#727169', '#e82424', '#98bb6c', '#e6c384', '#7fb4ca', '#938aa9', '#7aa89f', '#dcd7ba'],
    '#2d4f67',
  ),
  'kanagawa-lotus': paletteTheme(
    'Kanagawa Lotus',
    'light',
    '#f2ecbc',
    '#545464',
    ['#545464', '#c84053', '#6f894e', '#77713f', '#4d699b', '#624c83', '#597b75', '#f2ecbc'],
    ['#8a8980', '#d7474b', '#6e915f', '#836f4a', '#6693bf', '#76608d', '#6a9589', '#ffffff'],
    '#e6dfc4',
  ),
  'rose-pine': paletteTheme(
    'Rosé Pine',
    'dark',
    '#191724',
    '#e0def4',
    ['#26233a', '#eb6f92', '#9ccfd8', '#f6c177', '#31748f', '#c4a7e7', '#ebbcba', '#e0def4'],
    ['#6e6a86', '#eb6f92', '#9ccfd8', '#f6c177', '#31748f', '#c4a7e7', '#ebbcba', '#e0def4'],
    '#403d52',
  ),
  palenight: paletteTheme(
    'Palenight',
    'dark',
    '#292d3e',
    '#a6accd',
    ['#292d3e', '#f07178', '#c3e88d', '#ffcb6b', '#82aaff', '#c792ea', '#89ddff', '#d0d0d0'],
    ['#676e95', '#f07178', '#c3e88d', '#ffcb6b', '#82aaff', '#c792ea', '#89ddff', '#ffffff'],
    '#3b3f51',
  ),
  horizon: paletteTheme(
    'Horizon',
    'dark',
    '#1c1e26',
    '#bdc0c2',
    ['#1c1e26', '#e95678', '#29d398', '#fab795', '#26bbd9', '#ee64ac', '#59e1e3', '#d5d8da'],
    ['#6f6f70', '#ec6a88', '#3fdaa4', '#fac29b', '#3fc4de', '#f075b5', '#6be4e6', '#e5e5e5'],
    '#2e303e',
  ),
  'atom-one-light': paletteTheme(
    'Atom One Light',
    'light',
    '#fafafa',
    '#383a42',
    ['#383a42', '#e45649', '#50a14f', '#c18401', '#4078f2', '#a626a4', '#0184bc', '#fafafa'],
    ['#696c77', '#e45649', '#50a14f', '#c18401', '#4078f2', '#a626a4', '#0184bc', '#ffffff'],
    '#e5e5e6',
  ),
  'github-light': paletteTheme(
    'GitHub Light',
    'light',
    '#ffffff',
    '#24292e',
    ['#24292e', '#d73a49', '#22863a', '#b08800', '#0366d6', '#6f42c1', '#1b7c83', '#f6f8fa'],
    ['#586069', '#cb2431', '#34d058', '#dbab09', '#2188ff', '#8a63d2', '#3192aa', '#ffffff'],
    '#c8e1ff',
  ),
  'material-light': paletteTheme(
    'Material Light',
    'light',
    '#fafafa',
    '#546e7a',
    ['#000000', '#e53935', '#91b859', '#f6a434', '#6182b8', '#7c4dff', '#39adb5', '#ffffff'],
    ['#90a4ae', '#e53935', '#91b859', '#f6a434', '#6182b8', '#7c4dff', '#39adb5', '#ffffff'],
    '#e7e8eb',
  ),
  'papercolor-light': paletteTheme(
    'PaperColor Light',
    'light',
    '#eeeeee',
    '#444444',
    ['#eeeeee', '#af0000', '#008700', '#5f8700', '#0087af', '#878787', '#005f87', '#444444'],
    ['#bcbcbc', '#d70000', '#d70087', '#af8700', '#0087d7', '#af005f', '#005faf', '#005f87'],
    '#d0d0d0',
  ),
  'zenburn-light': paletteTheme(
    'Zenburn Light',
    'light',
    '#f0efd0',
    '#4f4f4f',
    ['#3f3f3f', '#cc9393', '#7f9f7f', '#f0dfaf', '#8cd0d3', '#dc8cc3', '#93e0e3', '#dcdccc'],
    ['#6f6f6f', '#dca3a3', '#9fc59f', '#f5e6b3', '#94d7d9', '#e3a8d1', '#a6e8eb', '#ffffff'],
    '#d6d6b8',
  ),
  'smyck-light': paletteTheme(
    'Smyck Light',
    'light',
    '#f7f7f7',
    '#202020',
    ['#000000', '#b44b4b', '#287373', '#a0742b', '#518fba', '#a565a5', '#2b9aa0', '#d0d0d0'],
    ['#626262', '#cf6a6a', '#3a9a9a', '#c79b4b', '#78b4d0', '#c58bc5', '#5fc4c4', '#ffffff'],
    '#d9e2e6',
  ),
  'base2tone-light': paletteTheme(
    'Base2Tone Light',
    'light',
    '#f7f7f7',
    '#383838',
    ['#383838', '#b75555', '#6c8f6c', '#a67c52', '#5f7f9f', '#8e709f', '#5a9696', '#eeeeee'],
    ['#707070', '#d47a7a', '#91b691', '#c59b70', '#86a8c8', '#af91c0', '#82c1c1', '#ffffff'],
    '#e2e2e2',
  ),
  'flat-remix-light': paletteTheme(
    'Flat Remix Light',
    'light',
    '#f5f5f5',
    '#242424',
    ['#242424', '#d81e00', '#5ea702', '#cfae00', '#427ab3', '#89658e', '#00aba4', '#dbded8'],
    ['#777777', '#f54235', '#99e343', '#fdeb61', '#84b0d8', '#b88fc1', '#58e7e1', '#ffffff'],
    '#d7e3f0',
  ),
  'selenized-light': paletteTheme(
    'Selenized Light',
    'light',
    '#fbf3db',
    '#53676d',
    ['#53676d', '#d2212d', '#489100', '#ad8900', '#0072d4', '#ca4898', '#009c8f', '#ffffff'],
    ['#8299a1', '#d2212d', '#489100', '#ad8900', '#0072d4', '#ca4898', '#009c8f', '#ffffff'],
    '#e5dfc5',
  ),
  'ocean-light': paletteTheme(
    'Ocean Light',
    'light',
    '#f2f7fc',
    '#334455',
    ['#334455', '#cc4b4b', '#3b8c6e', '#b58a3c', '#3979bc', '#9465ad', '#328f9c', '#f4f7fa'],
    ['#718292', '#e36c6c', '#64b895', '#d4ad63', '#6da3d5', '#b78bca', '#65bbc4', '#ffffff'],
    '#d9e7f3',
  ),
  'tango-light': paletteTheme(
    'Tango Light',
    'light',
    '#ffffff',
    '#2e3436',
    ['#2e3436', '#cc0000', '#4e9a06', '#c4a000', '#3465a4', '#75507b', '#06989a', '#d3d7cf'],
    ['#555753', '#ef2929', '#8ae234', '#fce94f', '#729fcf', '#ad7fa8', '#34e2e2', '#eeeeec'],
    '#e8e8e8',
  ),
}

export const TERMINAL_THEMES: Record<TerminalThemeId, TerminalTheme> = {
  'default-light': localizedBuiltinTheme(
    base({
      name: 'default-light',
      type: 'light',
      background: '#fbfbfc',
      foreground: '#25252a',
      cursor: '#71717a',
      cursorAccent: '#fbfbfc',
      selectionBackground: '#d4d4d85c',
      black: '#18181b',
      red: '#dc2626',
      green: '#16a34a',
      yellow: '#ca8a04',
      blue: '#2563eb',
      magenta: '#a21caf',
      cyan: '#0891b2',
      white: '#e4e4e7',
      brightBlack: '#71717a',
      brightRed: '#ef4444',
      brightGreen: '#22c55e',
      brightYellow: '#eab308',
      brightBlue: '#3b82f6',
      brightMagenta: '#d946ef',
      brightCyan: '#06b6d4',
      brightWhite: '#fafafa',
    }),
    'terminal.theme.defaultLight',
  ),
  'default-dark': localizedBuiltinTheme(
    base({
      name: 'default-dark',
      type: 'dark',
      background: '#0e0e10',
      foreground: '#dedee3',
      cursor: '#a1a1aa',
      cursorAccent: '#0e0e10',
      selectionBackground: '#3f3f465c',
      black: '#18181b',
      red: '#f87171',
      green: '#4ade80',
      yellow: '#facc15',
      blue: '#60a5fa',
      magenta: '#e879f9',
      cyan: '#22d3ee',
      white: '#d4d4d8',
      brightBlack: '#71717a',
      brightRed: '#fca5a5',
      brightGreen: '#86efac',
      brightYellow: '#fde047',
      brightBlue: '#93c5fd',
      brightMagenta: '#f0abfc',
      brightCyan: '#67e8f9',
      brightWhite: '#fafafa',
    }),
    'terminal.theme.defaultDark',
  ),
  'rose-pine-dawn': base({
    name: 'Rose Pine Dawn',
    type: 'light',
    background: '#faf4ed',
    foreground: '#575279',
    cursor: '#575279',
    cursorAccent: '#faf4ed',
    selectionBackground: '#dfdad9',
    black: '#575279',
    red: '#b4637a',
    green: '#286983',
    yellow: '#ea9d34',
    blue: '#56949f',
    magenta: '#907aa9',
    cyan: '#56949f',
    white: '#f2e9de',
    brightBlack: '#797593',
    brightRed: '#c26d85',
    brightGreen: '#31748f',
    brightYellow: '#f6b34b',
    brightBlue: '#639da7',
    brightMagenta: '#a68dbb',
    brightCyan: '#639da7',
    brightWhite: '#fffaf3',
  }),
  'flexoki-dark': base({
    name: 'Flexoki Dark',
    type: 'dark',
    background: '#100f0f',
    foreground: '#cecdc3',
    cursor: '#cecdc3',
    cursorAccent: '#100f0f',
    selectionBackground: '#403d38',
    black: '#100f0f',
    red: '#d14d41',
    green: '#879a39',
    yellow: '#d0a215',
    blue: '#4385be',
    magenta: '#8b7ec8',
    cyan: '#3aa99f',
    white: '#cecdc3',
    brightBlack: '#575653',
    brightRed: '#af3029',
    brightGreen: '#66800b',
    brightYellow: '#ad8301',
    brightBlue: '#205ea6',
    brightMagenta: '#5e409d',
    brightCyan: '#24837b',
    brightWhite: '#e6e4d9',
  }),
  ...ADDITIONAL_TERMINAL_THEMES,
}

export const TERMINAL_THEME_OPTIONS = Object.entries(TERMINAL_THEMES).map(([value, theme]) => ({
  value: value as TerminalThemeId,
  label: theme.name,
  type: theme.type,
}))

function parseCustomTerminalThemes(raw: string): Array<TerminalTheme & { id: string }> {
  try {
    const value: unknown = JSON.parse(raw || '[]')
    return Array.isArray(value)
      ? value.filter((item): item is TerminalTheme & { id: string } =>
          Boolean(
            item &&
            typeof item === 'object' &&
            typeof (item as { id?: unknown }).id === 'string' &&
            typeof (item as { name?: unknown }).name === 'string',
          ),
        )
      : []
  } catch {
    return []
  }
}

export function loadCustomTerminalThemes(): Array<TerminalTheme & { id: string }> {
  return parseCustomTerminalThemes(readCachedPreference(PREFERENCE_KEYS.terminalThemes))
}

export async function loadCustomTerminalThemesFromPreferences(): Promise<Array<TerminalTheme & { id: string }>> {
  return parseCustomTerminalThemes(await getPreference(PREFERENCE_KEYS.terminalThemes))
}

export function saveCustomTerminalThemes(themes: Array<TerminalTheme & { id: string }>): void {
  const value = JSON.stringify(themes)
  if (readCachedPreference(CUSTOM_TERMINAL_THEMES_KEY) !== value)
    void setPreference(PREFERENCE_KEYS.terminalThemes, value).catch(() => undefined)
}
export function getTerminalTheme(id: TerminalThemeId, fallback: TerminalThemeId): TerminalTheme {
  return TERMINAL_THEMES[id] ?? loadCustomTerminalThemes().find((theme) => theme.id === id) ?? TERMINAL_THEMES[fallback]
}
