// Seasonal Effects Engine for Jackie Memorial
class SeasonalEngine {
    constructor() {
        this.canvas = document.getElementById('seasonal-canvas');
        this.ctx = this.canvas.getContext('2d');
        this.particles = [];
        this.season = 'winter';
        this.animationId = null;
        this.resize();

        window.addEventListener('resize', () => this.resize());
        this.detectSeason();
        this.start();
    }

    resize() {
        this.canvas.width = window.innerWidth;
        this.canvas.height = window.innerHeight;
    }

    detectSeason() {
        const month = new Date().getMonth() + 1;
        if (month >= 3 && month <= 5) this.season = 'spring';
        else if (month >= 6 && month <= 8) this.season = 'summer';
        else if (month >= 9 && month <= 11) this.season = 'fall';
        else this.season = 'winter';

        document.body.className = this.season;
        this.updateBadge();
    }

    updateBadge() {
        const badge = document.getElementById('season-badge');
        const icons = { spring: '🌸', summer: '☀️', fall: '🍂', winter: '❄️' };
        badge.textContent = `${icons[this.season]} ${this.season.charAt(0).toUpperCase() + this.season.slice(1)}`;
    }

    setSeason(season) {
        this.season = season;
        document.body.className = season;
        this.particles = [];
        this.updateBadge();
    }

    start() {
        const animate = () => {
            this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
            this.updateParticles();
            this.drawParticles();
            this.animationId = requestAnimationFrame(animate);
        };
        animate();
    }

    updateParticles() {
        // Maintain particle count based on season
        const targetCount = {
            spring: 60,
            summer: 40,
            fall: 80,
            winter: 100
        }[this.season] || 50;

        while (this.particles.length < targetCount) {
            this.particles.push(this.createParticle());
        }

        // Update existing particles
        this.particles = this.particles.filter(p => {
            p.update();
            return p.life > 0;
        });
    }

    createParticle() {
        const creators = {
            spring: () => new PetalParticle(this.canvas.width, this.canvas.height),
            summer: () => new FireflyParticle(this.canvas.width, this.canvas.height),
            fall: () => new LeafParticle(this.canvas.width, this.canvas.height),
            winter: () => new SnowParticle(this.canvas.width, this.canvas.height)
        };
        return (creators[this.season] || creators.winter)();
    }

    drawParticles() {
        this.particles.forEach(p => p.draw(this.ctx));
    }

    stop() {
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
        }
    }
}

// Base Particle class
class Particle {
    constructor(w, h) {
        this.x = Math.random() * w;
        this.y = Math.random() * h;
        this.life = 1.0;
        this.size = Math.random() * 3 + 1;
    }

    update() {
        this.life -= 0.003;
    }

    draw(ctx) {
        ctx.globalAlpha = this.life;
    }
}

// Spring - Falling Petals
class PetalParticle extends Particle {
    constructor(w, h) {
        super(w, h);
        this.vx = Math.random() * 2 - 1;
        this.vy = Math.random() * 1 + 0.5;
        this.rotation = Math.random() * Math.PI * 2;
        this.rotationSpeed = Math.random() * 0.02 - 0.01;
        this.colors = ['#ffb7c5', '#ffc0cb', '#ffd1dc', '#ff69b4'];
        this.color = this.colors[Math.floor(Math.random() * this.colors.length)];
        this.y = -10; // Start above screen
    }

    update() {
        super.update();
        this.x += this.vx + Math.sin(this.y * 0.01) * 0.5;
        this.y += this.vy;
        this.rotation += this.rotationSpeed;
    }

    draw(ctx) {
        super.draw(ctx);
        ctx.save();
        ctx.translate(this.x, this.y);
        ctx.rotate(this.rotation);
        ctx.fillStyle = this.color;
        ctx.beginPath();
        ctx.ellipse(0, 0, this.size * 2, this.size, 0, 0, Math.PI * 2);
        ctx.fill();
        ctx.restore();
    }
}

// Summer - Fireflies
class FireflyParticle extends Particle {
    constructor(w, h) {
        super(w, h);
        this.vx = Math.random() * 2 - 1;
        this.vy = Math.random() * 2 - 1;
        this.glowPhase = Math.random() * Math.PI * 2;
        this.glowSpeed = Math.random() * 0.05 + 0.02;
    }

    update() {
        super.update();
        this.x += this.vx;
        this.y += this.vy;
        this.glowPhase += this.glowSpeed;

        // Wrap around screen
        if (this.x < 0) this.x = window.innerWidth;
        if (this.x > window.innerWidth) this.x = 0;
        if (this.y < 0) this.y = window.innerHeight;
        if (this.y > window.innerHeight) this.y = 0;
    }

    draw(ctx) {
        const glow = Math.sin(this.glowPhase) * 0.5 + 0.5;
        ctx.globalAlpha = this.life * glow;

        // Glow effect
        const gradient = ctx.createRadialGradient(this.x, this.y, 0, this.x, this.y, this.size * 4);
        gradient.addColorStop(0, 'rgba(255, 215, 0, 1)');
        gradient.addColorStop(0.5, 'rgba(255, 215, 0, 0.3)');
        gradient.addColorStop(1, 'rgba(255, 215, 0, 0)');

        ctx.fillStyle = gradient;
        ctx.beginPath();
        ctx.arc(this.x, this.y, this.size * 4, 0, Math.PI * 2);
        ctx.fill();
    }
}

// Fall - Falling Leaves
class LeafParticle extends Particle {
    constructor(w, h) {
        super(w, h);
        this.vx = Math.random() * 3 - 1.5;
        this.vy = Math.random() * 2 + 1;
        this.sway = Math.random() * 0.02;
        this.swayOffset = Math.random() * Math.PI * 2;
        this.colors = ['#d2691e', '#cd853f', '#8b4513', '#ff8c00', '#daa520'];
        this.color = this.colors[Math.floor(Math.random() * this.colors.length)];
        this.y = -10;
        this.rotation = Math.random() * Math.PI * 2;
    }

    update() {
        super.update();
        this.x += this.vx + Math.sin(this.y * this.sway + this.swayOffset) * 2;
        this.y += this.vy;
        this.rotation += 0.02;
    }

    draw(ctx) {
        super.draw(ctx);
        ctx.save();
        ctx.translate(this.x, this.y);
        ctx.rotate(this.rotation);
        ctx.fillStyle = this.color;

        // Draw leaf shape
        ctx.beginPath();
        ctx.moveTo(0, -this.size * 2);
        ctx.quadraticCurveTo(this.size, -this.size, 0, this.size * 2);
        ctx.quadraticCurveTo(-this.size, -this.size, 0, -this.size * 2);
        ctx.fill();
        ctx.restore();
    }
}

// Winter - Snowflakes
class SnowParticle extends Particle {
    constructor(w, h) {
        super(w, h);
        this.vx = Math.random() * 2 - 1;
        this.vy = Math.random() * 2 + 0.5;
        this.size = Math.random() * 3 + 1;
        this.y = -10;
    }

    update() {
        super.update();
        this.x += this.vx + Math.sin(this.y * 0.01) * 0.5;
        this.y += this.vy;
    }

    draw(ctx) {
        super.draw(ctx);
        ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
        ctx.beginPath();
        ctx.arc(this.x, this.y, this.size, 0, Math.PI * 2);
        ctx.fill();

        // Add subtle glow
        ctx.shadowColor = 'rgba(255, 255, 255, 0.5)';
        ctx.shadowBlur = 5;
        ctx.beginPath();
        ctx.arc(this.x, this.y, this.size * 0.5, 0, Math.PI * 2);
        ctx.fill();
        ctx.shadowBlur = 0;
    }
}

// Initialize
const seasonalEngine = new SeasonalEngine();

function toggleSeason() {
    const seasons = ['spring', 'summer', 'fall', 'winter'];
    const current = seasonalEngine.season;
    const next = seasons[(seasons.indexOf(current) + 1) % seasons.length];
    seasonalEngine.setSeason(next);
}
