(function() {
  let ws;
  let state = { ports: [], mappings: [], scanRanges: [], domainSuffix: 'localhost' };

  function connect() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(proto + '//' + location.host + '/ws');

    ws.onopen = function() {
      console.log('Portgate WS connected');
    };

    ws.onmessage = function(e) {
      const msg = JSON.parse(e.data);
      if (msg.type === 'update') {
        state.ports = msg.data.ports || [];
        state.mappings = msg.data.mappings || [];
        state.scanRanges = msg.data.scan_ranges || [];
        state.domainSuffix = msg.data.domain_suffix || 'localhost';
        render();
      }
    };

    ws.onclose = function() {
      console.log('Portgate WS disconnected, reconnecting...');
      setTimeout(connect, 2000);
    };
  }

  function render() {
    renderPorts();
    renderMappings();
    renderScanRanges();
    renderSuffix();
  }

  function renderPorts() {
    const el = document.getElementById('ports');
    if (!state.ports.length) {
      el.innerHTML = '<div class="empty">No ports discovered yet...</div>';
      return;
    }

    el.innerHTML = state.ports.map(function(p) {
      const detail = [p.serviceName, p.title].filter(Boolean).join(' — ');
      const mapped = state.mappings.find(function(m) { return m.targetPort === p.port; });
      const sourceBadge = p.source === 'manual'
        ? '<span class="source-badge manual">manual</span>'
        : '<span class="source-badge scan">scan</span>';
      const exePathHtml = p.exePath
        ? '<div class="exe-path" title="' + escapeHtml(p.exePath) + '">' + escapeHtml(p.exePath) + '</div>'
        : '';
      return '<div class="port-item">' +
        '<div class="port-info">' +
          '<span class="status-dot ' + (p.healthy ? 'online' : 'offline') + '"></span>' +
          '<span class="port-number">:' + p.port + '</span>' +
          sourceBadge +
          '<span class="port-detail">' + escapeHtml(detail) + '</span>' +
        '</div>' +
        exePathHtml +
        (mapped
          ? '<span class="mapping-domain">' + escapeHtml(mapped.domain) + '.' + escapeHtml(state.domainSuffix) + '</span>'
          : '<div class="map-form">' +
              '<input type="text" placeholder="subdomain" id="map-input-' + p.port + '" ' +
                'onkeydown="if(event.key===\'Enter\')mapDomain(' + p.port + ')">' +
              '<button class="btn btn-primary" onclick="mapDomain(' + p.port + ')">Map</button>' +
            '</div>'
        ) +
        (p.source === 'manual'
          ? '<button class="btn btn-danger btn-sm" onclick="removePort(' + p.port + ')">Remove</button>'
          : ''
        ) +
      '</div>';
    }).join('');
  }

  function renderMappings() {
    const el = document.getElementById('mappings');
    if (!state.mappings.length) {
      el.innerHTML = '<div class="empty">No domain mappings configured</div>';
      return;
    }

    el.innerHTML = state.mappings.map(function(m) {
      const port = state.ports.find(function(p) { return p.port === m.targetPort; });
      const online = port && port.healthy;
      const systemBadge = m.system
        ? '<span class="source-badge system">system</span>'
        : '';
      return '<div class="mapping-item">' +
        '<div class="mapping-info">' +
          '<span class="status-dot ' + (online ? 'online' : 'offline') + '"></span>' +
          '<span class="mapping-domain">' + escapeHtml(m.domain) + '.' + escapeHtml(state.domainSuffix) + '</span>' +
          systemBadge +
          '<span class="mapping-target">→ :' + m.targetPort + '</span>' +
        '</div>' +
        (m.system
          ? ''
          : '<button class="btn btn-danger" onclick="removeMapping(\'' + escapeHtml(m.domain) + '\')">Remove</button>'
        ) +
      '</div>';
    }).join('');
  }

  function renderScanRanges() {
    var el = document.getElementById('scan-ranges');
    if (!state.scanRanges.length) {
      el.innerHTML = '<div class="empty">No scan ranges configured</div>';
      return;
    }

    el.innerHTML = state.scanRanges.map(function(r) {
      return '<div class="range-item">' +
        '<span class="range-label">' + r.start + ' – ' + r.end + '</span>' +
        '<button class="btn btn-danger btn-sm" onclick="removeScanRange(' + r.start + ',' + r.end + ')">Remove</button>' +
      '</div>';
    }).join('');
  }

  function renderSuffix() {
    var input = document.getElementById('domain-suffix');
    var note = document.getElementById('suffix-note');
    var saveBtn = document.getElementById('save-suffix-btn');
    if (input && input !== document.activeElement) {
      input.value = state.domainSuffix;
    }
    if (note) {
      note.style.display = state.domainSuffix !== 'localhost' ? '' : 'none';
    }
    if (saveBtn && input && input !== document.activeElement) {
      saveBtn.style.display = 'none';
    }
  }

  // Show save button when suffix input changes
  document.addEventListener('DOMContentLoaded', function() {
    var input = document.getElementById('domain-suffix');
    if (input) {
      input.addEventListener('input', function() {
        var saveBtn = document.getElementById('save-suffix-btn');
        if (saveBtn) {
          saveBtn.style.display = input.value.trim() !== state.domainSuffix ? '' : 'none';
        }
      });
      input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') saveDomainSuffix();
      });
    }
  });

  window.saveDomainSuffix = function() {
    var input = document.getElementById('domain-suffix');
    var suffix = input.value.trim().toLowerCase();
    if (!suffix) {
      alert('Domain suffix cannot be empty');
      return;
    }
    fetch('/api/domain-suffix', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ suffix: suffix })
    }).then(function(r) {
      if (!r.ok) r.text().then(function(t) { alert('Error: ' + t); });
    });
  };

  window.addScanRange = function() {
    var startEl = document.getElementById('add-range-start');
    var endEl = document.getElementById('add-range-end');
    var start = parseInt(startEl.value, 10);
    var end = parseInt(endEl.value, 10);
    if (!start || !end || start < 1 || end > 65535 || start > end) {
      alert('Enter a valid range (1-65535, start <= end)');
      return;
    }
    fetch('/api/scan-ranges', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ start: start, end: end })
    }).then(function(r) {
      if (r.ok) { startEl.value = ''; endEl.value = ''; }
      else r.text().then(function(t) { alert('Error: ' + t); });
    });
  };

  window.removeScanRange = function(start, end) {
    fetch('/api/scan-ranges?start=' + start + '&end=' + end, {
      method: 'DELETE'
    });
  };

  window.addPort = function() {
    var portEl = document.getElementById('add-port-number');
    var nameEl = document.getElementById('add-port-name');
    var pathEl = document.getElementById('add-port-path');
    var port = parseInt(portEl.value, 10);
    if (!port || port < 1 || port > 65535) {
      alert('Enter a valid port number (1-65535)');
      return;
    }
    fetch('/api/ports', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ port: port, name: nameEl.value.trim(), path: pathEl.value.trim() })
    }).then(function(r) {
      if (r.ok) { portEl.value = ''; nameEl.value = ''; pathEl.value = ''; }
      else r.text().then(function(t) { alert('Error: ' + t); });
    });
  };

  window.mapDomain = function(port) {
    const input = document.getElementById('map-input-' + port);
    const domain = input.value.trim().toLowerCase();
    if (!domain) return;

    fetch('/api/mappings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ domain: domain, port: port })
    }).then(function(r) {
      if (!r.ok) r.text().then(function(t) { alert('Error: ' + t); });
    });
  };

  window.removeMapping = function(domain) {
    fetch('/api/mappings?domain=' + encodeURIComponent(domain), {
      method: 'DELETE'
    });
  };

  window.removePort = function(port) {
    fetch('/api/ports?port=' + port, {
      method: 'DELETE'
    });
  };

  function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }

  connect();
})();
