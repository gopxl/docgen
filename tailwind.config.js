/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./resources/views/**/*.gohtml'],
  theme: {
    colors: {
      "white": "#ffffff",
      "off-white": "#dae1e7",
      "black": "#000000",
      "primary": "#d01e57",
      "primary-dark": "#c10042",
      "secondary-extra-light": "#5b5f80",
      "secondary-light": "#454967",
      "secondary": "#383456",
      "secondary-dark": "#332f4d",
      "tertiary-light": "#f6f3e5",
      "tertiary": "#ddb799",

      "background": "#1B1D27",
      "sidebar": "#15161E",
      "codeblock": "#303445",
      "inline-code": "#393454",
      "alert": "#5b5f80",
    },
    fontFamily: {
      sans: ["sans-serif"],
      mono: ["monospace"],
    },
    extend: {},
  },
  plugins: [],
}

