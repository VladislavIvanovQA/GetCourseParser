/** Print styles for GetCourse lesson pages (hide chrome, keep content). */
const GC_PRINT_CSS = `
@media print {
  .gc-account-left-bar,
  .left-menu,
  #left-menu,
  .navbar-default,
  .navbar,
  .page-header-nav,
  .lesson-navigation,
  .comments-area,
  .comment-form-wrapper,
  .add-comment,
  .lesson-answers,
  .footer,
  .gc-footer,
  .modal,
  .popup,
  [class*="chat-widget"],
  [id*="chat"] {
    display: none !important;
  }
  .lesson-content,
  .standard-page-content,
  .gc-main-content,
  .content-wrapper {
    width: 100% !important;
    max-width: 100% !important;
  }
  img, video, iframe {
    max-width: 100% !important;
    page-break-inside: avoid;
  }
}
`;

function attachDebugger(tabId) {
  return new Promise((resolve, reject) => {
    chrome.debugger.attach({ tabId }, "1.3", () => {
      if (chrome.runtime.lastError) reject(new Error(chrome.runtime.lastError.message));
      else resolve();
    });
  });
}

function detachDebugger(tabId) {
  return new Promise((resolve) => {
    chrome.debugger.detach({ tabId }, () => resolve());
  });
}

function debuggerCommand(tabId, method, params = {}) {
  return new Promise((resolve, reject) => {
    chrome.debugger.sendCommand({ tabId }, method, params, (result) => {
      if (chrome.runtime.lastError) reject(new Error(chrome.runtime.lastError.message));
      else resolve(result);
    });
  });
}

async function printTabToPdfBase64(tabId) {
  await attachDebugger(tabId);
  try {
    await debuggerCommand(tabId, "Page.enable");
    const result = await debuggerCommand(tabId, "Page.printToPDF", {
      printBackground: true,
      preferCSSPageSize: false,
      paperWidth: 8.27,
      paperHeight: 11.69,
      marginTop: 0.4,
      marginBottom: 0.4,
      marginLeft: 0.4,
      marginRight: 0.4,
      scale: 0.95,
    });
    return result.data || "";
  } finally {
    await detachDebugger(tabId);
  }
}

globalThis.gcPrintPDF = {
  GC_PRINT_CSS,
  printTabToPdfBase64,
};
