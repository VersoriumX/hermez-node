const fs = require('fs');
const path = require('path');
const { ethers } = require('ethers');

async function main() {
    // Load JSON files
    const staticData1 = JSON.parse(fs.readFileSync(path.join(__dirname, '../data/static1.json'), 'utf8'));
    const staticData2 = JSON.parse(fs.readFileSync(path.join(__dirname, '../data/static2.json'), 'utf8'));

    // Example: Connect to Ethereum
    const provider = ethers.getDefaultProvider('mainnet'); // Change to your network
    const response = await provider.send(staticData1.method, staticData1.params);
    
    console.log(`Response for ${staticData1.method}:`, response);
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error(error);
        process.exit(1);
    });
