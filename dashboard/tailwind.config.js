/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        'bg-base':    '#0D1117',
        'bg-panel':   '#161B22',
        'bg-raised':  '#21262D',
        'border':     '#30363D',
        'border-str': '#444C56',
        'text-pri':   '#E6EDF3',
        'text-sec':   '#8B949E',
        'text-muted': '#6E7681',
        'accent':     '#58A6FF',
        'success':    '#3FB950',
        'warning':    '#D29922',
        'danger':     '#F85149',
        'purple':     '#BC8CFF',
      },
      fontFamily: {
        sans: ['Inter', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
    },
  },
  plugins: [],
}
