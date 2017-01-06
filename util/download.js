"use strict";

var page = require('webpage').create(),
    system = require('system'),
    fs = require('fs');

if (system.args.length < 2) {
  console.log('Usage: download.js URL');
  phantom.exit();
}

var address = system.args[1];

page.settings.loadImages = false;
page.settings.resourceTimeout = 10000;
page.settings.webSecurityEnabled = false;

page.onResourceRequested = function(requestData, request) {
  if ((/http:\/\/.+?\.css/gi).test(requestData['url'])) {
    request.abort();
  }
};

page.open(address, function(status) {
  if (status === 'success') {
    var html = '',
        count = page.framesCount + 1;

    for (var i = 0; i < count; i++) {
      // console.log(page.frameName);
      html += '\n\n' + page.frameContent + '\n\n';
      page.switchToMainFrame();
      page.switchToFrame(i);
    }

    var leioRes = {
      Err: '',
      Body: html
    };

    // var file = fs.open('./result.html', 'w');
    // file.write(html);
    // file.close();

    system.stdout.write(JSON.stringify(leioRes));
    phantom.exit();
  } else {
    console.log(JSON.stringify({Err: 'Phantomjs Error', Body: ''}));
    phantom.exit();
  }
});
