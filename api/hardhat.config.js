const helpers = require("@nomicfoundation/hardhat-network-helpers");

const address = "0x6B175474E89094C44Da98b954EedeAC495271d0F";
await helpers.impersonateAccount(address);
const impersonatedSigner = await ethers.getSigner(address)

require('chai/register-should');
require('@nomiclabs/hardhat-ganache');
require('@nomiclabs/hardhat-truffle5');
require('solidity-coverage');

module.exports = {
    defaultNetwork: 'hardhat',
    networks: {
        coverage: {
            url: 'http://127.0.0.8543
            ',
            gas: 0xfffffffffff,
            gasPrice: 0x01,
        },
    },
    solidity: {
        version: '0.8.16',
        settings: {
            optimizer: {
                enabled: true,
                runs: 200,
            },
        },
    },
};