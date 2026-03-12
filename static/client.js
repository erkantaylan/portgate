(function() {
  let ws;
  let state = { ports: [], mappings: [], scanRanges: [], domainSuffix: 'localhost' };

  var defaultFilters = { http: true, tcp: true, mapped: true, unmapped: true };
  var filters = (function() {
    try {
      var saved = JSON.parse(localStorage.getItem('portgate-filters'));
      if (saved && typeof saved === 'object') {
        var f = {};
        for (var k in defaultFilters) f[k] = saved[k] !== false;
        return f;
      }
    } catch(e) {}
    return JSON.parse(JSON.stringify(defaultFilters));
  })();

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

  function checkAuth(r) {
    if (r.status === 401) {
      window.location.href = '/login';
      return null;
    }
    return r;
  }

  fetch('/api/version').then(checkAuth).then(function(r) { return r && r.json(); }).then(function(d) {
    if (!d) return;
    var el = document.getElementById('version-tag');
    if (el && d.version) el.textContent = d.version;
  }).catch(function() {});

  function render() {
    renderPortFilters();
    renderPorts();
    renderMappings();
    renderScanRanges();
    renderSuffix();
  }

  function renderPortFilters() {
    var el = document.getElementById('port-filters');
    if (!el) return;
    var mappedSet = new Set(state.mappings.map(function(m) { return m.targetPort; }));
    var counts = { http: 0, tcp: 0, mapped: 0, unmapped: 0 };
    state.ports.forEach(function(p) {
      if (p.serviceName === 'http') counts.http++;
      else counts.tcp++;
      if (mappedSet.has(p.port)) counts.mapped++;
      else counts.unmapped++;
    });
    var labels = [
      { key: 'http', label: 'HTTP' },
      { key: 'tcp', label: 'TCP' },
      { key: 'mapped', label: 'Mapped' },
      { key: 'unmapped', label: 'Unmapped' }
    ];
    el.innerHTML = labels.map(function(f) {
      return '<label class="filter-checkbox' + (filters[f.key] ? ' active' : '') + '">' +
        '<input type="checkbox"' + (filters[f.key] ? ' checked' : '') +
        ' onchange="togglePortFilter(\'' + f.key + '\', this.checked)">' +
        '<span class="filter-label">' + f.label + '</span>' +
        '<span class="filter-count">' + counts[f.key] + '</span>' +
      '</label>';
    }).join('');
  }

  window.togglePortFilter = function(key, checked) {
    filters[key] = checked;
    try { localStorage.setItem('portgate-filters', JSON.stringify(filters)); } catch(e) {}
    renderPortFilters();
    renderPorts();
  };

  function renderPorts() {
    var el = document.getElementById('ports');
    var mappedSet = new Set(state.mappings.map(function(m) { return m.targetPort; }));
    var filtered = state.ports.filter(function(p) {
      var isMapped = mappedSet.has(p.port);
      var mappingOk = (isMapped && filters.mapped) || (!isMapped && filters.unmapped);
      var isHttp = p.serviceName === 'http';
      var typeOk = (isHttp && filters.http) || (!isHttp && filters.tcp);
      return mappingOk && typeOk;
    });
    if (!filtered.length) {
      el.innerHTML = '<div class="empty">No ports match current filters</div>';
      return;
    }

    el.innerHTML = filtered.map(function(p) {
      var isMapped = mappedSet.has(p.port);
      var detail = [p.serviceName, p.title].filter(Boolean).join(' — ');
      var sourceBadge = p.source === 'manual'
        ? '<span class="source-badge manual">manual</span>'
        : '<span class="source-badge scan">scan</span>';
      var mappedBadge = isMapped
        ? '<span class="source-badge mapped">mapped</span>'
        : '';
      var exePathHtml = p.exePath
        ? '<div class="exe-path" title="' + escapeHtml(p.exePath) + '">' + escapeHtml(p.exePath) + '</div>'
        : '';
      return '<div class="port-item">' +
        '<div class="port-info">' +
          '<span class="status-dot ' + (p.healthy ? 'online' : 'offline') + '"></span>' +
          '<span class="port-number">:' + p.port + '</span>' +
          sourceBadge +
          mappedBadge +
          '<span class="port-detail">' + escapeHtml(detail) + '</span>' +
        '</div>' +
        exePathHtml +
        (!isMapped
          ? '<button class="btn btn-primary btn-sm" onclick="openMapModal(' + p.port + ')">Map</button>'
          : ''
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
          '<a class="mapping-domain" href="http://' + escapeHtml(m.domain) + '.' + escapeHtml(state.domainSuffix) + '" target="_blank">' + escapeHtml(m.domain) + '.' + escapeHtml(state.domainSuffix) + '</a>' +
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

  window.openMapModal = function(port) {
    var existing = document.getElementById('map-modal');
    if (existing) existing.remove();

    var overlay = document.createElement('div');
    overlay.id = 'map-modal';
    overlay.className = 'modal-overlay';
    overlay.innerHTML =
      '<div class="modal">' +
        '<h3>Map port :' + port + ' to domain</h3>' +
        '<div class="modal-input-row">' +
          '<input type="text" id="map-modal-input" placeholder="subdomain" autofocus>' +
          '<span class="suffix-label">.' + escapeHtml(state.domainSuffix) + '</span>' +
        '</div>' +
        '<div class="modal-actions">' +
          '<button class="btn" onclick="closeMapModal()">Cancel</button>' +
          '<button class="btn btn-primary" onclick="submitMapModal(' + port + ')">Map</button>' +
        '</div>' +
      '</div>';

    overlay.addEventListener('click', function(e) {
      if (e.target === overlay) closeMapModal();
    });

    document.body.appendChild(overlay);
    setTimeout(function() { document.getElementById('map-modal-input').focus(); }, 0);

    document.getElementById('map-modal-input').addEventListener('keydown', function(e) {
      if (e.key === 'Enter') submitMapModal(port);
      if (e.key === 'Escape') closeMapModal();
    });
  };

  window.closeMapModal = function() {
    var el = document.getElementById('map-modal');
    if (el) el.remove();
  };

  window.submitMapModal = function(port) {
    var input = document.getElementById('map-modal-input');
    var domain = input.value.trim().toLowerCase();
    if (!domain) return;

    fetch('/api/mappings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ domain: domain, port: port })
    }).then(function(r) {
      if (!r.ok) r.text().then(function(t) { alert('Error: ' + t); });
      else closeMapModal();
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
