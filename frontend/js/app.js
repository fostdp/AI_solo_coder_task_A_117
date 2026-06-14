class App {
    constructor() {
        this.apiBase = '/api';
        this.waterwheels = [];
        this.selectedWheel = null;
        this.latestTelemetry = {};
        this.renderer = null;
        this.refreshInterval = null;
        this.speedLevel = 0;
        this.speedOptions = [1, 2, 5, 10];
        this.isPlaying = true;

        this.init();
    }

    init() {
        this.renderer = new WaterwheelRenderer('waterwheelCanvas');
        this.renderer.onClick = () => this.showDetailModal();

        this.bindEvents();
        this.loadWaterwheels();
        this.startDataRefresh();
    }

    bindEvents() {
        document.getElementById('playPauseBtn').addEventListener('click', () => this.togglePlay());
        document.getElementById('speedBtn').addEventListener('click', () => this.cycleSpeed());
        document.getElementById('chartRange').addEventListener('change', () => this.refreshEfficiencyChart());
        document.getElementById('runOptimizationBtn').addEventListener('click', () => this.runOptimization());
    }

    async loadWaterwheels() {
        try {
            const res = await fetch(`${this.apiBase}/waterwheels`);
            this.waterwheels = await res.json();
            this.renderWaterwheelList();
            this.updateHeaderStats();
        } catch (e) {
            console.error('加载筒车列表失败:', e);
        }
    }

    renderWaterwheelList() {
        const list = document.getElementById('waterwheelList');
        list.innerHTML = '';

        for (const wheel of this.waterwheels) {
            const telemetry = this.latestTelemetry[wheel.id];
            let statusClass = 'status-offline';
            if (telemetry) {
                const mechEff = telemetry.mechanical_efficiency ?? 0.5;
                const hydEff = telemetry.hydraulic_efficiency ?? 0.5;
                if (mechEff * hydEff > 0.35) {
                    statusClass = 'status-online';
                } else {
                    statusClass = 'status-warning';
                }
            }

            const item = document.createElement('div');
            item.className = 'wheel-item' + (this.selectedWheel?.id === wheel.id ? ' active' : '');
            item.innerHTML = `
                <div class="wheel-item-name">
                    <span class="wheel-item-status ${statusClass}"></span>${wheel.name}
                </div>
                <div class="wheel-item-loc">${wheel.location}</div>
            `;
            item.addEventListener('click', () => this.selectWaterwheel(wheel));
            list.appendChild(item);
        }
    }

    selectWaterwheel(wheel) {
        this.selectedWheel = wheel;
        this.renderer.setWaterwheel(wheel);
        document.getElementById('canvasTitle').textContent = `${wheel.name} - ${wheel.location}`;
        document.getElementById('playPauseBtn').disabled = false;
        document.getElementById('speedBtn').disabled = false;
        document.getElementById('runOptimizationBtn').disabled = false;

        this.renderWaterwheelList();
        this.loadWheelData();
    }

    async loadWheelData() {
        if (!this.selectedWheel) return;

        await Promise.all([
            this.loadLatestTelemetry(),
            this.refreshEfficiencyChart(),
            this.loadAnalysis(),
            this.loadAlerts(),
            this.loadOptimizations()
        ]);
    }

    async loadLatestTelemetry() {
        if (!this.selectedWheel) return;
        try {
            const res = await fetch(`${this.apiBase}/waterwheels/${this.selectedWheel.id}/telemetry?limit=1`);
            const data = await res.json();
            if (data && data.length > 0) {
                this.latestTelemetry[this.selectedWheel.id] = data[0];
                this.renderer.setTelemetry(data[0]);
                this.updateMetrics(data[0]);
            }
        } catch (e) {
            console.error('加载遥测数据失败:', e);
        }
    }

    updateMetrics(data) {
        document.getElementById('metricSpeed').textContent = data.rotation_speed?.toFixed(2) ?? '--';
        document.getElementById('metricLift').textContent = data.water_lift?.toFixed(1) ?? '--';
        document.getElementById('metricDrop').textContent = data.water_level_drop?.toFixed(2) ?? '--';
        document.getElementById('metricFlow').textContent = data.flow_velocity?.toFixed(2) ?? '--';

        const mechEff = (data.mechanical_efficiency ?? 0) * 100;
        const hydEff = (data.hydraulic_efficiency ?? 0) * 100;
        document.getElementById('metricMechEff').textContent = mechEff.toFixed(1);
        document.getElementById('metricHydEff').textContent = hydEff.toFixed(1);
    }

    async refreshEfficiencyChart() {
        if (!this.selectedWheel) return;

        const hours = parseInt(document.getElementById('chartRange').value);
        const end = new Date();
        const start = new Date(end.getTime() - hours * 3600 * 1000);

        try {
            const res = await fetch(
                `${this.apiBase}/waterwheels/${this.selectedWheel.id}/telemetry/range?start=${encodeURIComponent(start.toISOString())}&end=${encodeURIComponent(end.toISOString())}`
            );
            const data = await res.json();
            this.drawEfficiencyChart(data);
        } catch (e) {
            console.error('加载效率曲线失败:', e);
        }
    }

    drawEfficiencyChart(data) {
        const canvas = document.getElementById('efficiencyChart');
        const rect = canvas.getBoundingClientRect();
        const dpr = window.devicePixelRatio || 1;
        canvas.width = rect.width * dpr;
        canvas.height = 200 * dpr;
        const ctx = canvas.getContext('2d');
        ctx.scale(dpr, dpr);

        const w = rect.width;
        const h = 200;
        const padL = 45, padR = 15, padT = 15, padB = 30;
        const cw = w - padL - padR;
        const ch = h - padT - padB;

        ctx.clearRect(0, 0, w, h);

        ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
        ctx.lineWidth = 1;
        for (let i = 0; i <= 4; i++) {
            const y = padT + (ch / 4) * i;
            ctx.beginPath();
            ctx.moveTo(padL, y);
            ctx.lineTo(w - padR, y);
            ctx.stroke();

            ctx.fillStyle = '#78909c';
            ctx.font = '11px sans-serif';
            ctx.textAlign = 'right';
            ctx.fillText(((4 - i) * 25) + '%', padL - 6, y + 4);
        }

        if (!data || data.length === 0) {
            ctx.fillStyle = '#78909c';
            ctx.font = '14px sans-serif';
            ctx.textAlign = 'center';
            ctx.fillText('暂无数据', w / 2, h / 2);
            return;
        }

        const drawLine = (getter, color) => {
            const validData = data.filter(d => getter(d) != null);
            if (validData.length < 2) return;

            const grad = ctx.createLinearGradient(0, padT, 0, padT + ch);
            grad.addColorStop(0, color + '88');
            grad.addColorStop(1, color + '00');

            ctx.strokeStyle = color;
            ctx.lineWidth = 2;
            ctx.beginPath();
            validData.forEach((d, i) => {
                const x = padL + (i / (validData.length - 1)) * cw;
                const y = padT + ch - getter(d) * ch;
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            });
            ctx.stroke();
        };

        drawLine(d => d.mechanical_efficiency ?? 0, '#4fc3f7');
        drawLine(d => d.hydraulic_efficiency ?? 0, '#66bb6a');
        drawLine(d => (d.mechanical_efficiency ?? 0) * (d.hydraulic_efficiency ?? 0), '#ffb74d');

        ctx.font = '11px sans-serif';
        const legendY = 5;
        const items = [
            { label: '机械效率', color: '#4fc3f7' },
            { label: '水力效率', color: '#66bb6a' },
            { label: '综合效率', color: '#ffb74d' }
        ];
        let lx = padL;
        for (const item of items) {
            ctx.fillStyle = item.color;
            ctx.fillRect(lx, legendY, 12, 12);
            ctx.fillStyle = '#b0bec5';
            ctx.textAlign = 'left';
            ctx.fillText(item.label, lx + 16, legendY + 10);
            lx += ctx.measureText(item.label).width + 40;
        }

        if (data.length > 0) {
            ctx.fillStyle = '#78909c';
            ctx.font = '10px sans-serif';
            ctx.textAlign = 'left';
            const firstTime = new Date(data[0].time);
            ctx.fillText(firstTime.toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit' }), padL, h - 8);

            ctx.textAlign = 'right';
            const lastTime = new Date(data[data.length - 1].time);
            ctx.fillText(lastTime.toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit' }), w - padR, h - 8);
        }
    }

    async loadAnalysis() {
        if (!this.selectedWheel) return;
        try {
            const res = await fetch(`${this.apiBase}/waterwheels/${this.selectedWheel.id}/efficiency`);
            const analysis = await res.json();
            this.renderAnalysis(analysis);
        } catch (e) {
            console.error('加载分析数据失败:', e);
        }
    }

    renderAnalysis(analysis) {
        const container = document.getElementById('analysisContent');
        if (!analysis) {
            container.innerHTML = '<div class="analysis-empty">暂无分析数据</div>';
            return;
        }

        const formatNum = (n, unit = '') => {
            if (n == null) return '--';
            if (Math.abs(n) < 100) return n.toFixed(2) + unit;
            return n.toFixed(1) + unit;
        };

        const getEffClass = (v) => {
            if (v >= 0.6) return 'good';
            if (v >= 0.4) return 'warning';
            return 'danger';
        };

        const mechEff = analysis.mechanical_efficiency ?? 0;
        const hydEff = analysis.hydraulic_efficiency ?? 0;
        const overall = analysis.overall_efficiency ?? 0;

        container.innerHTML = `
            <div class="analysis-grid">
                <div class="analysis-item">
                    <div class="analysis-item-label">输入功率</div>
                    <div class="analysis-item-value">${formatNum(analysis.input_power, ' W')}</div>
                </div>
                <div class="analysis-item">
                    <div class="analysis-item-label">输出功率</div>
                    <div class="analysis-item-value">${formatNum(analysis.output_power, ' W')}</div>
                </div>
                <div class="analysis-item">
                    <div class="analysis-item-label">输入转矩</div>
                    <div class="analysis-item-value">${formatNum(analysis.torque_input, ' N·m')}</div>
                </div>
                <div class="analysis-item">
                    <div class="analysis-item-label">输出转矩</div>
                    <div class="analysis-item-value">${formatNum(analysis.torque_output, ' N·m')}</div>
                </div>
                <div class="analysis-item">
                    <div class="analysis-item-label">提水阻力</div>
                    <div class="analysis-item-value">${formatNum(analysis.lift_resistance, ' N·m')}</div>
                </div>
                <div class="analysis-item">
                    <div class="analysis-item-label">转速</div>
                    <div class="analysis-item-value">${formatNum(analysis.rotation_speed, ' rpm')}</div>
                </div>
                <div class="analysis-item">
                    <div class="analysis-item-label">机械效率</div>
                    <div class="analysis-item-value ${getEffClass(mechEff)}">${(mechEff * 100).toFixed(1)}%</div>
                </div>
                <div class="analysis-item">
                    <div class="analysis-item-label">水力利用率</div>
                    <div class="analysis-item-value ${getEffClass(hydEff)}">${(hydEff * 100).toFixed(1)}%</div>
                </div>
                <div class="analysis-item" style="grid-column: span 2;">
                    <div class="analysis-item-label">综合效率</div>
                    <div class="analysis-item-value ${getEffClass(overall)}" style="font-size: 1.4rem;">${(overall * 100).toFixed(1)}%</div>
                </div>
            </div>
        `;
    }

    async loadAlerts() {
        if (!this.selectedWheel) return;
        try {
            const res = await fetch(`${this.apiBase}/waterwheels/${this.selectedWheel.id}/alerts?limit=20`);
            const alerts = await res.json();
            this.renderAlerts(alerts);
        } catch (e) {
            console.error('加载告警失败:', e);
        }
    }

    renderAlerts(alerts) {
        const container = document.getElementById('alertsList');
        if (!alerts || alerts.length === 0) {
            container.innerHTML = '<div class="analysis-empty">暂无告警记录</div>';
            return;
        }

        container.innerHTML = alerts.map(a => `
            <div class="alert-item">
                <div class="alert-item-time">${new Date(a.time).toLocaleString('zh-CN')}</div>
                <div class="alert-item-msg">${a.message}</div>
            </div>
        `).join('');
    }

    async loadOptimizations() {
        if (!this.selectedWheel) return;
        try {
            const res = await fetch(`${this.apiBase}/waterwheels/${this.selectedWheel.id}/optimizations?limit=10`);
            const results = await res.json();
            this.renderOptimizations(results);
        } catch (e) {
            console.error('加载优化记录失败:', e);
        }
    }

    renderOptimizations(results) {
        const container = document.getElementById('optimizationList');
        if (!results || results.length === 0) {
            container.innerHTML = '<div class="analysis-empty">暂无优化记录</div>';
            return;
        }

        container.innerHTML = results.map(r => {
            const params = r.bucket_shape_params || {};
            return `
                <div class="opt-item">
                    <div class="opt-item-time">${new Date(r.created_at).toLocaleString('zh-CN')}</div>
                    <div class="opt-item-detail">
                        优化提水量: <strong>${r.optimized_lift_rate.toFixed(1)} m³/h</strong>
                        (提升 ${r.improvement_percent.toFixed(1)}%)<br>
                        水斗布置角: ${r.bucket_angle.toFixed(1)}° | 代数: ${r.generation_count}
                    </div>
                </div>
            `;
        }).join('');
    }

    async runOptimization() {
        if (!this.selectedWheel) return;
        const btn = document.getElementById('runOptimizationBtn');
        btn.disabled = true;
        btn.textContent = '优化中...';

        try {
            const res = await fetch(`${this.apiBase}/waterwheels/${this.selectedWheel.id}/optimize`, { method: 'POST' });
            if (res.ok) {
                await this.loadOptimizations();
            }
        } catch (e) {
            console.error('运行优化失败:', e);
        } finally {
            btn.disabled = false;
            btn.textContent = '运行结构优化';
        }
    }

    showDetailModal() {
        if (!this.selectedWheel) return;

        const wheel = this.selectedWheel;
        const telemetry = this.latestTelemetry[wheel.id];

        document.getElementById('modalTitle').textContent = wheel.name;
        document.getElementById('modalBody').innerHTML = `
            <div class="analysis-grid" style="margin-bottom: 20px;">
                <div class="analysis-item">
                    <div class="analysis-item-label">位置</div>
                    <div class="analysis-item-value">${wheel.location}</div>
                </div>
                <div class="analysis-item">
                    <div class="analysis-item-label">直径</div>
                    <div class="analysis-item-value">${wheel.diameter} m</div>
                </div>
                <div class="analysis-item">
                    <div class="analysis-item-label">水斗数量</div>
                    <div class="analysis-item-value">${wheel.bucket_count} 个</div>
                </div>
                <div class="analysis-item">
                    <div class="analysis-item-label">单斗容量</div>
                    <div class="analysis-item-value">${wheel.bucket_capacity * 1000} L</div>
                </div>
                <div class="analysis-item" style="grid-column: span 2;">
                    <div class="analysis-item-label">最大提水量</div>
                    <div class="analysis-item-value">${wheel.max_flow_rate} m³/h</div>
                </div>
            </div>
            ${telemetry ? `
                <h3 style="color: #4fc3f7; margin-bottom: 12px; font-size: 1rem;">实时遥测</h3>
                <div class="analysis-grid">
                    <div class="analysis-item">
                        <div class="analysis-item-label">转速</div>
                        <div class="analysis-item-value">${telemetry.rotation_speed?.toFixed(2)} rpm</div>
                    </div>
                    <div class="analysis-item">
                        <div class="analysis-item-label">提水量</div>
                        <div class="analysis-item-value">${telemetry.water_lift?.toFixed(1)} m³/h</div>
                    </div>
                    <div class="analysis-item">
                        <div class="analysis-item-label">水位落差</div>
                        <div class="analysis-item-value">${telemetry.water_level_drop?.toFixed(2)} m</div>
                    </div>
                    <div class="analysis-item">
                        <div class="analysis-item-label">水流流速</div>
                        <div class="analysis-item-value">${telemetry.flow_velocity?.toFixed(2)} m/s</div>
                    </div>
                    <div class="analysis-item">
                        <div class="analysis-item-label">机械效率</div>
                        <div class="analysis-item-value">${((telemetry.mechanical_efficiency ?? 0) * 100).toFixed(1)}%</div>
                    </div>
                    <div class="analysis-item">
                        <div class="analysis-item-label">水力效率</div>
                        <div class="analysis-item-value">${((telemetry.hydraulic_efficiency ?? 0) * 100).toFixed(1)}%</div>
                    </div>
                </div>
            ` : '<div class="analysis-empty">暂无遥测数据</div>'}
        `;

        document.getElementById('detailModal').classList.remove('hidden');
    }

    togglePlay() {
        this.isPlaying = !this.isPlaying;
        this.renderer.setRunning(this.isPlaying);
        document.getElementById('playPauseBtn').textContent = this.isPlaying ? '⏸ 暂停' : '▶ 播放';
    }

    cycleSpeed() {
        this.speedLevel = (this.speedLevel + 1) % this.speedOptions.length;
        const speed = this.speedOptions[this.speedLevel];
        this.renderer.setSpeed(speed);
        document.getElementById('speedBtn').textContent = speed + 'x 速度';
    }

    startDataRefresh() {
        this.refreshInterval = setInterval(() => this.refreshData(), 5000);
    }

    async refreshData() {
        await this.loadLatestTelemetry();
        this.renderWaterwheelList();
        this.updateHeaderStats();

        if (this.selectedWheel) {
            const now = new Date();
            const lastRefresh = this._lastChartRefresh || 0;
            if (now.getTime() - lastRefresh > 60000) {
                this.refreshEfficiencyChart();
                this.loadAlerts();
                this._lastChartRefresh = now.getTime();
            }
        }
    }

    updateHeaderStats() {
        document.getElementById('totalWheels').textContent = this.waterwheels.length;

        let online = 0;
        let totalAlerts = 0;
        for (const wheel of this.waterwheels) {
            if (this.latestTelemetry[wheel.id]) {
                online++;
            }
        }
        document.getElementById('onlineWheels').textContent = online;
        document.getElementById('alertCount').textContent = totalAlerts;
    }
}

document.addEventListener('DOMContentLoaded', () => {
    window.app = new App();
});
