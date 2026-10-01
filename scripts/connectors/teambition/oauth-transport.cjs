'use strict';
const http = require('node:http');
const https = require('node:https');
const { randomBytes } = require('node:crypto');
// The native CLI requires an OS keyring for OAuth. This per-command loopback
// adapter preserves its discovery/command parser while the platform owns OAuth.
// The local routing nonce is never sent to Teambition; only the real Bearer grant is.
async function startOAuthTransport(accessToken, send = https.request) {
  const routingToken = 'u-' + randomBytes(24).toString('hex');
  const requests = new Set();
  const server = http.createServer((incoming, outgoing) => {
    if (incoming.url !== '/api/mcp/v2' || !['POST', 'GET', 'DELETE'].includes(incoming.method) ||
        incoming.headers.authorization !== 'Bearer ' + routingToken) {
      outgoing.writeHead(403).end(); return;
    }
    const headers = { authorization: 'Bearer ' + accessToken };
    for (const name of ['accept', 'content-type', 'mcp-session-id', 'mcp-protocol-version']) {
      if (incoming.headers[name]) headers[name] = incoming.headers[name];
    }
    const upstream = send({ hostname: 'open.teambition.com', port: 443, path: '/api/mcp/v2', method: incoming.method, headers }, (response) => {
      const publicHeaders = {};
      for (const name of ['content-type', 'mcp-session-id']) if (response.headers[name]) publicHeaders[name] = response.headers[name];
      outgoing.writeHead(response.statusCode, publicHeaders);
      response.on('error', () => outgoing.destroy()); response.pipe(outgoing);
    });
    requests.add(upstream);
    upstream.setTimeout(120000, () => upstream.destroy());
    upstream.on('close', () => requests.delete(upstream));
    upstream.on('error', () => { if (!outgoing.headersSent) outgoing.writeHead(502); outgoing.end(); });
    let bytes = 0;
    incoming.on('data', data => { bytes += data.length; if (bytes > 4 * 1024 * 1024) { upstream.destroy(); incoming.destroy(); } });
    incoming.on('error', () => upstream.destroy());
    outgoing.on('close', () => { if (!outgoing.writableFinished) upstream.destroy(); });
    incoming.pipe(upstream);
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  return { origin: 'http://127.0.0.1:' + server.address().port, routingToken,
    close: () => new Promise(resolve => { for (const request of requests) request.destroy(); server.close(resolve); server.closeAllConnections(); }) };
}
module.exports = { startOAuthTransport };
