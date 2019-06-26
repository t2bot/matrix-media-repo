const request = require("request-promise");
const thumbnails = require("./thumbnails.json");
const fs = require("fs");

const target = "http://localhost:8001";

async function download(mxc) {
    console.log("Downloading " + mxc);
    const result = await request(`${target}/_matrix/media/r0/thumbnail/${mxc.substring("mxc://".length)}?width=800&height=600&method=crop&animated=false`, {encoding: null});
    fs.writeFileSync("expected_thumbnails/" + mxc.substring("mxc://".length).replace(/\//g, '_') + ".png", result);
}

console.log("Downloading thumbnails...");
const promises = [];
for (const thumbnailMxc of Object.keys(thumbnails)) {
    promises.push(download(thumbnailMxc));
}

Promise.all(promises).then(() => console.log("Done!"));
