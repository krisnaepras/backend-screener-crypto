/** @type {import('tailwindcss').Config} */
export default {
    content: [
        "./frontend/index.html",
        "./frontend/src/**/*.{js,ts,jsx,tsx}",
    ],
    theme: {
        extend: {
            colors: {
                background: "#1B2028", // Main BG
                surface: "#232732",    // Lighter than BG
                surfaceHover: "#2C313C", // Even lighter for Hover
                surfaceHighlight: "#31353F",
                primary: "#3A6FF8",
                secondary: "#1ECB4F",
                danger: "#F46D22",
                warning: "#FFC01E",
                info: "#64CFF9",
                white: "#FFFFFF", // Explicitly add white to extend to be safe
                text: {
                    primary: "#FFFFFF",
                    secondary: "#9E9E9E",
                    muted: "#6b7280"
                }
            },
            fontFamily: {
                sans: ['Poppins', 'Inter', 'sans-serif'],
                display: ['Inter', 'sans-serif'],
            },
            boxShadow: {
                'card': '4px 4px 33px rgba(0, 0, 0, 0.05)',
                'glow': '4px 4px 32px rgba(103, 90, 255, 0.07)',
            }
        },
    },
    plugins: [],
}
