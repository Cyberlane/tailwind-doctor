module.exports = {
  prefix: "tw-",
  plugins: [require("@tailwindcss/typography")],
  theme: {
    extend: {
      colors: {
        brand: {
          500: "#3b82f6",
        },
      },
      spacing: {
        gutter: "1.5rem",
      },
      borderRadius: {
        card: "0.75rem",
      },
    },
  },
}
