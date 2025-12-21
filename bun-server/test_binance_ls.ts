
const FAPI_BASE_URL = "https://fapi.binance.com";
const symbol = "BOBUSDT";
const control = "BTCUSDT";

async function check(s: string) {
    try {
        console.log(`Checking ${s} (Corrected URL)...`);
        // Change from /fapi/v1/ to /futures/data/
        const url = `${FAPI_BASE_URL}/futures/data/topLongShortAccountRatio?symbol=${s}&period=5m&limit=1`;
        const res = await fetch(url);
        console.log(`Status: ${res.status}`);
        if (res.ok) {
            const data = await res.json();
            console.log("Data:", JSON.stringify(data, null, 2));
        } else {
            console.log("Body:", (await res.text()).slice(0, 500));
        }
    } catch (e) {
        console.error(`${s} Error:`, e);
    }
}

await check(control);
await check(symbol);
