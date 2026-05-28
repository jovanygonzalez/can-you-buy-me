### Can you buy me?

### Contexto del Proyecto: Plataforma de Subastas "Drop" en Tiempo Real

**TÍTULO DE REFERENCIA:** Plataforma de Subastas de Ultra-Baja Latencia (Modelo Drop)

**DESCRIPCIÓN DEL NEGOCIO (ESTADO ACTUAL - PMV):**
El producto es una plataforma web de subastas en vivo diseñada para soportar ráfagas masivas de tráfico y concurrencia. Operamos bajo un modelo de "Drops" (eventos de alta escasez programados a una hora específica, subastando artículos de alto valor como coleccionables, arte o *sneakers* de edición limitada).

El PMV está enfocado en validar la tracción en un solo país (México) con el flujo más directo posible:

1. **Captura de Usuarios:** Registro básico y validación de métodos de pago mediante Stripe Setup Intents (sin cobro ni retención automática inicial).
2. **El Evento en Vivo:** A la hora programada, miles de usuarios conectados envían pujas que deben registrarse y reflejarse en la pantalla de todos los participantes en submilisegundos, garantizando un orden cronológico inmaculado.
3. **Resolución:** El cierre de la subasta y el cobro al ganador se gestionan con procesos manuales asistidos por el administrador.

**STACK TÉCNICO ESTRICTO (Reglas para el Agente):**

* **Frontend:** Flutter Web puro (sin intermediarios REST pesados).
* **Backend:** Lenguaje Go (microservicios o monolito modular).
* **Comunicación Cliente-Servidor (Escritura):** gRPC-Web (el frontend envía las pujas directamente al backend en Go).
* **Comunicación Servidor-Cliente (Lectura/Tiempo Real):** WebSockets conectados directamente a **NATS JetStream** (el backend publica eventos validados en NATS, y NATS empuja las actualizaciones a los miles de clientes en Flutter simultáneamente).
* **Caché de Lectura:** Redis (ElastiCache) para servir el catálogo estático y evitar saturar la base de datos durante las ráfagas de tráfico (*Thundering Herd*).
* **Base de Datos Principal:** PostgreSQL (RDS) para persistencia de usuarios, pagos y auditoría final.
* **Infraestructura:** AWS (ECS/Fargate o EC2, S3 + CloudFront).

**VISIÓN DE FUTURO Y ESCALABILIDAD (Hacia dónde evoluciona):**
El PMV sentará las bases para una arquitectura global basada en Event Sourcing y CQRS. La visión a futuro contempla:

1. **Cruce de Fronteras Multipaís:** Expansión a toda Latinoamérica con un catálogo segmentado. Un clúster central de Go gestionando identidades globales, mientras que el enrutamiento geográfico masivo se delega a la partición lógica de temas (*Subjects*) dentro de superclústeres de NATS.
2. **Bidding Inteligente:** Integración de retenciones de fondos dinámicas cruzadas y automatización algorítmica de resolución de empates.
3. **Auditoría Criptográfica:** Uso del registro inmutable de NATS JetStream como fuente única de la verdad (*Source of Truth*) para garantizar transparencia absoluta en el ecosistema de pujas y resolver disputas legales o financieras de forma instantánea.



### Can you buy me? - PRODUCTO MÍNIMO VIABLE

Lanzar un Producto Mínimo Viable (PMV) para validar una idea de mercado requiere ser táctico: el objetivo no es escribir el código más escalable del mundo desde el día uno, sino construir el "bucle de valor" principal lo más rápido posible. Si los usuarios están dispuestos a meter su tarjeta de crédito y pujar, la idea está validada. El resto se puede automatizar después.

Aquí tienes el plan de ataque técnico priorizado por ruta crítica (de lo más vital a lo periférico) para lanzar rápido en México, asumiendo que **tú y tu equipo administrarán gran parte del sistema de forma manual** por detrás.

---

### FASE 1: La Confianza y el Dinero (Prioridad Crítica)

Si los usuarios no pueden registrarse ni dejar su método de pago de forma segura, no hay negocio. Esta es la primera pieza que debe funcionar.

**1. Base de Datos y Registro (PostgreSQL)**

* **Alcance PMV:** Olvídate de perfiles complejos. Solo necesitas: Nombre, Email, Password encriptado (bcrypt) y un ID de usuario.
* **Acción:** Levantar una base de datos pequeña en AWS (RDS PostgreSQL).
* **Backend (Go):** Crear el endpoint de registro/login por gRPC o REST básico y devolver un JWT.

**2. Integración de Pagos (Stripe)**

* **El Reto:** Nadie debe pujar si no tiene fondos, pero cobrar por adelantado espanta a los usuarios.
* **Alcance PMV (Manual/Híbrido):** * Usar **Stripe Setup Intents**. Cuando el usuario se registra, la app de Flutter le pide guardar su tarjeta. Stripe la valida y guarda de forma segura.
* **Cero automatización de cobro:** Durante el PMV, el backend *no* hace cargos automáticos ni retenciones previas (autorizaciones) complejas al hacer cada puja, porque requiere mucha lógica. Solo verificas que Stripe tenga una tarjeta válida asociada a ese ID de usuario para dejarlo entrar a la subasta.
* El cobro al ganador se hará **manualmente** desde el Dashboard de Stripe al día siguiente.



### FASE 2: El Motor de Subasta (El Núcleo del Valor)

Aquí es donde entra la tecnología avanzada que discutimos, pero recortada a su mínima expresión funcional.

**3. El Catálogo (Redis + Carga Manual)**

* **Alcance PMV:** Nada de paneles de administración para subir artículos.
* **Acción:** Levantar ElastiCache (Redis) en AWS. Tú o tu equipo insertarán los datos del artículo a subastar (título, precio base, URL de la imagen alojada en S3) **directamente en Redis o Postgres escribiendo scripts manuales** o queries de SQL.
* **Frontend (Flutter Web):** Lee los datos del artículo a través del backend en Go.

**4. El Sistema de Pujas (NATS + Go)**

* **Alcance PMV:** El flujo de tiempo real de ida y vuelta.
* **Acción:** Desplegar un contenedor de NATS JetStream (un solo nodo en EC2 para el PMV es suficiente, luego lo escalas al clúster de 3 nodos).
* **Backend (Go):** Recibe la puja por gRPC, verifica contra la memoria que supere la puja anterior, y publica en NATS.
* **Frontend (Flutter Web):** Se conecta por WebSocket a NATS para escuchar la lluvia de pujas y actualizar el número en pantalla gigante.
* **Lo Manual:** Iniciar y detener el reloj de la subasta no será automático. El backend tendrá un endpoint secreto oculto (ej. `/admin/close-auction`) que tú ejecutarás manualmente con Postman o cURL cuando consideres que la subasta en vivo terminó. Esto publica el evento final en NATS y detiene las pujas.

### FASE 3: Infraestructura y Entrega

Cómo se sostiene esto en la nube de la forma más barata y rápida posible para validar.

**5. Arquitectura AWS (Versión Lean)**

* **Frontend:** Flutter Web compilado y alojado en **Amazon S3** con **CloudFront** (CDN). Cuesta centavos y aguanta tráfico infinito.
* **Backend & Mensajería:** Un clúster pequeño de **Amazon ECS (Fargate)** o simplemente dos instancias **EC2**. Una para correr tu binario de Go y otra para el binario de NATS JetStream.
* **Datos:** RDS (Postgres, la instancia más pequeña) y ElastiCache (Redis nodo básico).

### FASE 4: La Periferia (Lo Menos Crítico)

Estas son piezas que los usuarios esperan, pero que no validan el modelo de negocio. Se hacen al final o con herramientas externas de bajo código (No-Code).

**6. Contacto y Soporte**

* **Alcance PMV:** No programes un sistema de tickets.
* **Acción:** En la UI de Flutter, pon un botón de "Contacto" que abra un simple `mailto:` hacia tu correo, o integra un formulario de Google Forms / Typeform incrustado. Si insistes en hacerlo nativo, el backend de Go recibe el mensaje del formulario y usa un servicio gratuito como SendGrid o Amazon SES para enviarte el correo a ti. No guardes estos mensajes en tu base de datos todavía.

---

### Resumen del Flujo del Usuario en el PMV

1. **Entra:** Entra a la web en Flutter.
2. **Se Registra:** Crea cuenta (Go + Postgres).
3. **Habilita Puja:** Ingresa tarjeta (Stripe guarda el método de pago).
4. **Espera:** Ve el artículo (Go + Redis).
5. **Puja:** Dispara gRPC a Go, Go publica en NATS, NATS empuja el precio a todos (WebSockets).
6. **Fin:** Tú cierras la subasta manualmente (Postman $\rightarrow$ Go $\rightarrow$ NATS). La pantalla dice "Subasta finalizada".
7. **Operación post-venta (Tú):** Vas a tu base de datos, ves quién ganó, abres el panel de Stripe y le haces el cargo a su tarjeta guardada. Le envías un correo manual felicitándolo.

Este enfoque recorta semanas de desarrollo en paneles de administración, lógica de retención de fondos, crons automáticos y gestión de imágenes. Te permite centrar todo el código de Go y Flutter estrictamente en la experiencia de la puja, que es lo que realmente importa para enganchar al mercado.