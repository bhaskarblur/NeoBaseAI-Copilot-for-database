/** @type {import('tailwindcss').Config} */
export default {
  mode: "jit",
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        primary: {
          yellow: '#FFDB58',
        },
        neo: {
          gray: '#F0F0F0',
          error: '#FF6B6B',
          success: '#90EE90'
        }
      },
      fontFamily: {
        sans: ['Public Sans', 'sans-serif'],
      },
    },
  },
  plugins: [],
};
