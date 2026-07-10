// assets/charts.js — ECharts visualizations for ecat refactoring report
(function() {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var rule = style.getPropertyValue('--rule').trim();
  var bg2 = style.getPropertyValue('--bg2').trim();

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
          { value: 74.4, itemStyle: { color: accent + '99' } },
          { value: 84.8, itemStyle: { color: accent + 'bb' } },
          { value: 86.9, itemStyle: { color: accent + 'cc' } },
          { value: 89.2, itemStyle: { color: accent + 'dd' } },
          { value: 90.3, itemStyle: { color: accent } },
          { value: 91.5, itemStyle: { color: accent2 } }
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
          { value: 5.0, itemStyle: { color: accent2 } },
          { value: 3.3, itemStyle: { color: accent2 } },
          { value: 0, itemStyle: { color: accent } },
          { value: 0, itemStyle: { color: accent } },
          { value: 0, itemStyle: { color: accent } }
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

  // Mermaid init
  if (typeof mermaid !== 'undefined') {
    mermaid.initialize({
      startOnLoad: true,
      theme: 'dark',
      securityLevel: 'loose',
      themeVariables: {
        primaryColor: '#161b22',
        primaryTextColor: '#e6edf3',
        primaryBorderColor: '#30363d',
        lineColor: '#58a6ff',
        secondaryColor: '#161b22',
        tertiaryColor: '#0d1117'
      }
    });
  }
})();