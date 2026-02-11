Based on the market research and your specific constraints (low maintenance, high margin, geo-arbitrage), here are the requirements for the **NMN ROI Ranker**.

I have tagged items as **** for the MVP (Minimum Viable Product to start earning money) or **** for future updates.

### 1. Functional Requirements (What the system does)

These features directly drive value for the user (saving money/ensuring quality) and revenue for you (affiliate clicks).

**A. Core Calculation Engine**

* ** Price Per Gram Calculator:** The system must automatically calculate the base price per gram: `Price ÷ (Total Capsules × mg per capsule)`. This is the "honest" baseline users cannot find on Amazon.
* ** "Effective Cost" Algorithm:** The system must apply a "Bioavailability Coefficient" to calculate the *True Cost*.
* *Logic:* `Base Price Per Gram ÷ Bioavailability Multiplier = Effective Cost`.
* *Default Multipliers (configurable in backend):*
* Standard Powder/Capsules: **1.0** (Baseline - utilizing Slc12a8 transporter).
* Liposomal: **1.5 - 2.0** (Adjustable based on latest studies; currently debated but marketed as higher).
* Sublingual: **1.1** (Marginal speed benefit).


* ** Currency Normalization:** Automatically convert all prices to a single display currency (USD) for comparison, or detect user location to display EUR/GBP.

**B. Data & Trust Signals**

* ** CoA Verification Boolean:** A simple "Pass/Fail" check for Certificates of Analysis.
* *Rule:* If a brand provides a CoA from an ISO 17025 accredited lab (e.g., Eurofins, Micro Quality Labs) dated within the last 12 months = **PASS**.
* *Display:* A green checkmark or "Verified" badge next to the product.


* ** Purity Filter:** Filter out products with purity <99% or unknown purity.
* ** Heavy Metal Flag:** A visual warning if a CoA shows heavy metals (lead/mercury) near the safe threshold, even if "passing."

**C. Traffic & Revenue Features**

* ** Smart Affiliate Routing (Geo-Arbitrage):** The "Buy Now" button must dynamically route the user based on their IP address.
* *User in US:* Route to **Renue by Science** or **ProHealth** (USD Store).
* *User in UK:* Route to **NMN Bio** (UK Store) to avoid customs fees.
* *User in EU:* Route to **MoleQlar** or approved cross-border vendors (due to Novel Food regulations).


* ** Programmatic "Vs" Pages:** The system must generate static comparison pages for SEO traffic.
* *Structure:* `/compare/renue-vs-doublewood-nmn`
* *Content:* Auto-populated table comparing Price, Purity, and Shipping for those two specific brands.



**D. User Interface (The "Mercenary Accountant" Look)**

* ** Sortable Ranking Table:** Users must be able to sort by "Lowest Price per Gram," "Highest Purity," or "Best ROI."
* ** Dosage Calculator:** A simple input field ("I weigh 80kg") that updates the "Monthly Cost" column based on a recommended dosage (e.g., 500mg/day).

---

### 2. Non-Functional Requirements (System Quality)

These ensure the "low maintenance" and "low cost" aspects of your request.

**A. Data Integrity & Automation**

* ** 24-Hour Price Freshness:** Prices must be updated at least once every 24 hours via cron job scripts. If a scraper fails, the system should retain the last known price but flag it as "Unverified Today."
* ** "Zero-Maintenance" Architecture:** The site must be built as a **Static Site** (Next.js/Hugo) hosted on a CDN (Vercel/Netlify). No database servers to manage; just a JSON file updated by GitHub Actions.

**B. Compliance & Legal**

* ** EU "Novel Food" Disclaimer:** For users visiting from EU IP addresses, display a banner: *"NMN is classified as a Novel Food in the EU. Listings are for research/personal import purposes only."* This protects your affiliate accounts from being banned by strict EU regulators.
* ** FDA Disclaimer:** Standard footer text required for US compliance ("Not intended to diagnose, treat, cure...").

**C. Performance**

* ** Instant Load Time:** The table must render in <1 second. Speed is a trust signal for this demographic.
* ** Mobile Optimization:** 60% of traffic will be mobile; the massive data table must be scrollable or collapsible on small screens.

---

### 3. The MVP Strategy Summary

**Don't Build:** User accounts, forums, "Stack tracking," or complex health journals. SuppCo already does this.

**Do Build:** A ruthless, single-page spreadsheet that answers **one** question: *"Who has the cheapest authentic NMN today?"*

| Feature | Priority | Implementation Strategy (Lowest Cost) |
| --- | --- | --- |
| **Price Scraper** | **CRITICAL** | Python/Go script running on GitHub Actions (Free). |
| **ROI Calculator** | **CRITICAL** | Simple JavaScript formula on the frontend. |
| **CoA Verification** | **CRITICAL** | Manual entry for first 30 brands (don't automate yet). |
| **Affiliate Routing** | **CRITICAL** | GeniusLink or simple JS redirection logic. |
| **Price History Chart** | *Nice-to-Have* | Skip for launch. Adds database complexity. |
| **Email Capture** | *Nice-to-Have* | "Alert me when NMN drops below $0.50/gram." |
