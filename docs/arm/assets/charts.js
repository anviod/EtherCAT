(function() {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var rule = style.getPropertyValue('--rule').trim();
  var bg2 = style.getPropertyValue('--bg2').trim();

  // --- Chart 1: FrameOverlay series ARM64 comparison ---
  var chart1 = echarts.init(document.getElementById('chart-overlay-compare'), null, { renderer: 'svg' });
  var overlayData = [
    { name: 'FrameOverlay (original)', ns: 498.5, alloc: 2 },
    { name: 'FrameOverlayReuse (pool)', ns: 26.34, alloc: 0 },
    { name: 'FrameOverlayPool (helper)', ns: 26.0, alloc: 0 }
  ];
  var option1 = {
    tooltip: {
      trigger: 'axis',
      formatter: '{b}: {c} ns/op<br/>Allocations: {d}/op'
    },
    grid: { left: 10, right: 10, top: 30, bottom: 30, containLabel: true },
    xAxis: {
      type: 'category',
      data: overlayData.map(function(d) { return d.name; }),
      axisLabel: { color: ink },
      axisLine: { lineStyle: { color: rule } }
    },
    yAxis: {
      type: 'value',
      name: 'nanoseconds / op',
      axisLabel: { color: ink },
      axisLine: { lineStyle: { color: rule } },
      splitLine: { lineStyle: { color: '#f0f0f0' } }
    },
    series: [{
      type: 'bar',
      data: overlayData.map(function(d) {
        return {
          value: d.ns,
          itemStyle: {
            color: d.alloc === 0 ? accent : accent + '60'
          }
        };
      }),
      label: {
        show: true,
        position: 'top',
        formatter: '{c} ns/op',
        color: ink
      }
    }],
    animation: false
  };
  chart1.setOption(option1);
  window.addEventListener('resize', function() { chart1.resize(); });

  // --- Chart 2: Full ecfr comparison x86 vs ARM64 ---
  var chart2 = echarts.init(document.getElementById('chart-full-compare'), null, { renderer: 'svg' });
  var fullData = [
    { name: 'DatagramHeaderOverlay', x86: 2.61, arm: 7.75 },
    { name: 'DatagramHeaderCommit', x86: 3.30, arm: 5.06 },
    { name: 'DatagramOverlay', x86: 4.62, arm: 14.29 },
    { name: 'DatagramCommit', x86: 3.17, arm: 9.22 },
    { name: 'ETHFrameWriteDown', x86: 1.66, arm: 5.25 },
    { name: 'FrameOverlay', x86: 55.57, arm: 498.5 },
    { name: 'FrameOverlayReuse', x86: 9.70, arm: 26.34 },
    { name: 'FrameNewDatagram', x86: 141.7, arm: 810.5 },
    { name: 'FrameCommit', x86: 827.1, arm: 139.5 }
  ];
  var categories = fullData.map(function(d) { return d.name; });
  var option2 = {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' }
    },
    legend: {
      data: ['x86 (i5-13500H)', 'ARM64 (Cortex-A55)'],
      bottom: 0,
      textStyle: { color: ink }
    },
    grid: { left: 100, right: 20, top: 30, bottom: 70, containLabel: true },
    xAxis: {
      type: 'log',
      name: 'nanoseconds / op (log scale)',
      nameLocation: 'middle',
      nameGap: 30,
      axisLabel: { color: ink },
      axisLine: { lineStyle: { color: rule } },
      splitLine: { lineStyle: { color: '#f0f0f0' } }
    },
    yAxis: {
      type: 'category',
      data: categories,
      axisLabel: { color: ink, fontFamily: 'JetBrains Mono' },
      axisLine: { lineStyle: { color: rule } }
    },
    series: [
      {
        name: 'x86 (i5-13500H)',
        type: 'bar',
        data: fullData.map(function(d) { return d.x86; }),
        itemStyle: { color: accent2 },
        barWidth: '40%'
      },
      {
        name: 'ARM64 (Cortex-A55)',
        type: 'bar',
        data: fullData.map(function(d) { return d.arm; }),
        itemStyle: { color: accent },
        barWidth: '40%'
      }
    ],
    animation: false
  };
  chart2.setOption(option2);
  window.addEventListener('resize', function() { chart2.resize(); });

})();