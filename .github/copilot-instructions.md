# COPILOT INSTRUCTIONS

## 1. Tech Stack & Architecture
- **Backend:** Go (Scrapers, Math Engine, Normalization).
- **Frontend:** Next.js (App Router, SSG, Tailwind).
- **CI/CD:** GitHub Actions (cron jobs).
- **Hosting:** Vercel (or Cloudflare Pages).
- **Database:** NONE. The system uses local `.json` files in the `/data` directory as the sole source of truth. Do not suggest SQL, Prisma, or ORMs.
- **Architecture:** "Git-Scraper". Go scrapes and writes JSON. Next.js reads JSON at build time.

## 2. Coding Directives
- **Overrides, not OCR:** If a scraper misses data (like bottle size), DO NOT suggest OCR or image parsing. Fix it by adding a hardcoded override in `data/vendor_rules.json`.
- **String Isolation:** Keep HTML (`BodyHTML`) strictly separated from Type classification logic in `analyzer.go`.
- **Simplicity:** No complex state management on the frontend. It is a static HTML table.

## 3. Documentation Sync Protocol
**CRITICAL:** After modifying code, you MUST evaluate and update the documentation.
1. Scan `SPEC.md` and `README.md`.
2. Remove outdated logic, resolved TODOs, and fixed bugs.
3. Document the new state of the code using blunt, technical language. Do not describe what the code *will* do; describe what it *does*.