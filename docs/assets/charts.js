// assets/charts.js — ECharts visualizations for EtherCAT refactoring report (EdgeX gold theme)
(function() {
  var style = getComputedStyle(document.documentElement);
  var gold = style.getPropertyValue('--gold').trim() || '#c5a059';
  var goldDeep = style.getPropertyValue('--gold-deep').trim() || '#b38f43';
  var goldLight = style.getPropertyValue('--gold-light').trim() || '#dfc38a';
  var ink = style.getPropertyValue('--ink').trim() || '#343a40';
  var muted = style.getPropertyValue('--muted').trim() || '#6c757d';
  var rule = style.getPropertyValue('--line').trim() || '#e9ecef';

  // Gold palette: light to deep
  var goldPalette = ['#dfc38a', '#cfaa6a', '#c5a059', '#b38f43', '#9c7a35', '#7a5e28'];

  // --- Chart 1: Coverage ---
  var covEl = document.getElementById('chart-coverage');
  if (covEl) {
    var chart1 = echarts.init(covEl, null, { renderer: 'svg' });
    chart1.setOption({
      animation: false,
      tooltip: {
        trigger: 'axis',
        appendToBody: true,
        axisPointer: { type: 'shadow' },
        formatter: function(params) {
          return params[0].name + '<br/>覆盖率: ' + params[0].value + '%';
        }
      },
      grid: { left: '12%', right: '8%', top: '3%', bottom: '3%', containLabel: true },
      xAxis: {
        type: 'value',
        min: 0, max: 100,
        axisLabel: { color: muted, formatter: '{value}%' },
        splitLine: { lineStyle: { color: rule } },
        axisLine: { lineStyle: { color: rule } }
      },
      yAxis: {
        type: 'category',
        data: ['internal/link/udp', 'ecfr', 'ecmd', 'internal/sim', 'eni', 'internal/marshalling', 'ecee'],
        axisLabel: { color: ink, fontFamily: 'JetBrains Mono, monospace', fontSize: 12 },
        axisLine: { lineStyle: { color: rule } }
      },
      series: [{
        type: 'bar',
        data: [
          { value: 27.2, itemStyle: { color: muted } },
          { value: 74.4, itemStyle: { color: goldPalette[0] } },
          { value: 84.8, itemStyle: { color: goldPalette[1] } },
          { value: 86.9, itemStyle: { color: goldPalette[2] } },
          { value: 89.2, itemStyle: { color: goldPalette[3] } },
          { value: 90.3, itemStyle: { color: goldPalette[4] } },
          { value: 91.5, itemStyle: { color: goldPalette[5] } }
        ],
        label: {
          show: true,
          position: 'right',
          color: ink,
          fontSize: 12,
          fontWeight: 600,
          formatter: '{c}%'
        },
        barWidth: 22
      }]
    });
    window.addEventListener('resize', function() { chart1.resize(); });
  }

  // --- Chart 2: Zero-allocation verification ---
  var perfEl = document.getElementById('chart-perf');
  if (perfEl) {
    var chart2 = echarts.init(perfEl, null, { renderer: 'svg' });
    chart2.setOption({
      animation: false,
      tooltip: {
        trigger: 'axis',
        appendToBody: true,
        axisPointer: { type: 'shadow' }
      },
      grid: { left: '18%', right: '8%', top: '3%', bottom: '3%', containLabel: true },
      xAxis: {
        type: 'value',
        name: 'ns/op',
        nameTextStyle: { color: muted },
        axisLabel: { color: muted },
        splitLine: { lineStyle: { color: rule } },
        axisLine: { lineStyle: { color: rule } }
      },
      yAxis: {
        type: 'category',
        data: [
          'DatagramOverlay',
          'DatagramCommit',
          'FrameOverlay',
          'FrameNewDatagram',
          'ETHFrameWriteDown'
        ],
        axisLabel: { color: ink, fontFamily: 'JetBrains Mono, monospace', fontSize: 11 },
        axisLine: { lineStyle: { color: rule } }
      },
      series: [{
        type: 'bar',
        data: [
          { value: 5.0, itemStyle: { color: goldDeep } },
          { value: 3.3, itemStyle: { color: goldDeep } },
          { value: 0, itemStyle: { color: goldPalette[0] } },
          { value: 0, itemStyle: { color: goldPalette[0] } },
          { value: 0, itemStyle: { color: goldPalette[0] } }
        ],
        label: {
          show: true,
          position: 'right',
          color: ink,
          fontSize: 11,
          formatter: function(p) {
            return p.value > 0 ? p.value.toFixed(2) + ' ns' : '0 allocs';
          }
        },
        barWidth: 20
      }]
    });
    window.addEventListener('resize', function() { chart2.resize(); });
  }

  // Mermaid init — neutral theme with gold accent
  if (typeof mermaid !== 'undefined') {
    mermaid.initialize({
      startOnLoad: true,
      theme: 'neutral',
      securityLevel: 'loose',
      themeVariables: {
        primaryColor: '#ffffff',
        primaryTextColor: '#343a40',
        primaryBorderColor: '#e9ecef',
        lineColor: '#b38f43',
        secondaryColor: '#f8f9fa',
        tertiaryColor: '#f8f9fa',
        fontSize: '14px'
      }
    });
  }
})();