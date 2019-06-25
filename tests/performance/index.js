const request = require("request-promise");
const thumbnails = require("./thumbnails.json");
const fs = require("fs");
const mkdirp = require("mkdirp");

const target = "http://localhost:8001";

const requestTimes = [];
let failed = 0;
let corrupted = 0;

async function thumbnailDefaults(mxc) {
    const start = (new Date()).getTime();

    const result = Buffer.from(await request(`${target}/_matrix/media/r0/thumbnail/${mxc.substring("mxc://".length)}?width=800&height=600&method=crop&animated=false`, {encoding: null}));
    const expectedResult = fs.readFileSync("expected_thumbnails/" + mxc.substring("mxc://".length).replace(/\//, '_') + ".png");

    if (Buffer.compare(result, expectedResult) !== 0) {
        console.warn("Unexpected corruption!");
        corrupted++;
    }

    return (new Date()).getTime() - start;
}

function handleThumbnail(mxc) {
    console.log("Starting download of " + mxc);
    return thumbnailDefaults(mxc).then(t => requestTimes.push(t)).catch(() => failed++);
}

const promises = [];
for (const thumbnailMxc of Object.keys(thumbnails)) {
    for (let i = 0; i < thumbnails[thumbnailMxc]; i++) {
        promises.push(handleThumbnail(thumbnailMxc));
    }
}

console.log("Waiting for results...");
Promise.all(promises).then(() => {
    requestTimes.sort();

    let average = 0;
    for (const i of requestTimes) average += i;
    average = average / requestTimes.length;

    const obj = {
        failed,
        corrupted,
        requestTimes,
        min: Math.min(...requestTimes),
        max: Math.max(...requestTimes),
        average,
        median: requestTimes[Math.floor(requestTimes.length / 2)],
        count: requestTimes.length,
    };

    console.log("Saving report...");
    mkdirp.sync("reports");
    fs.writeFileSync(`reports/${(new Date()).getTime()}.json`, JSON.stringify(obj, null, 2));

    delete obj.requestTimes;
    console.log(obj);
    console.log("Done!");
});
