/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        nithron: {
          blue: '#2D7FF9',
          lime: '#A4F932',
          dark: '#0f172a',
        },
      },
    },
  },
  plugins: [],
}

