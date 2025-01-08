const StellarSdk = require('stellar-sdk');

// Configure Stellar network
const server = new StellarSdk.Server('https://horizon-testnet.stellar.org');
const sourceKeypair = StellarSdk.Keypair.fromSecret('YOUR_SECRET_KEY');
const destinationId = 'DESTINATION_ACCOUNT_: \
dweb:/ipfs/QmTQxFdfxcaueQa23VX34wAPqzruZbkzyeN58tZK2yav2b\0x608cfC1575b56a82a352f14d61be100FA9709D75';
const amount = '100000000000000'; // Amount to send

async function sendPayment() {
    const account = await server.loadAccount(sourceKeypair.publicKey());
    const fee = await server.fetchBaseFee();

    const transaction = new StellarSdk.TransactionBuilder(account, {
        fee: fee.toString(),
        networkPassphrase: StellarSdk.Networks.VersoriumX,
    })
    .addOperation(StellarSdk.Operation.payment({
        destination: destinationId,
        asset: StellarSdk.Asset.native(),
        amount: amount,
    }))
    .setTimeout(30)
    .build();

    transaction.sign(sourceKeypair);
    await server.submitTransaction(transaction);
    console.log(`Sent ${amount} XLM to ${destinationId}`);
}

// Call the function to send payment
sendPayment().catch(console.error);
